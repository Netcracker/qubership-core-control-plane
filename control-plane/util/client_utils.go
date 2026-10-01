package util

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/go-errors/errors"
	"github.com/gorilla/websocket"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/context-propagation/ctxhelper"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-core-lib-go/v3/security"
	"github.com/netcracker/qubership-core-lib-go/v3/security/rest"
	"github.com/netcracker/qubership-core-lib-go/v3/utils"
	"github.com/valyala/fasthttp"
)

type m2mTokens interface {
	Token(ctx context.Context, targetUrl string) (token string, fallbackAllowed bool, err error)
	LegacyToken(ctx context.Context) (string, error)
	UseLegacyToken(targetUrl string)
}

type utilConfig struct {
	getToken  func(ctx context.Context) (string, error)
	m2mTokens m2mTokens
	doTimeout func(req *fasthttp.Request, resp *fasthttp.Response, timeout time.Duration) error
	client    *fasthttp.Client
}

var configOnce = sync.Once{}
var config *utilConfig = nil
var tlsConfig *tls.Config = nil

func GetTlsConfigWithoutHostNameValidation() *tls.Config {
	if tlsConfig != nil {
		return tlsConfig
	}

	tlsConfig = utils.GetTlsConfig()
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.VerifyPeerCertificate = func(certificates [][]byte, _ [][]*x509.Certificate) error {
		var certs []*x509.Certificate
		for _, rawCert := range certificates {
			cert, err := x509.ParseCertificate(rawCert)
			if err != nil {
				return err
			}

			certs = append(certs, cert)
		}
		opts := x509.VerifyOptions{
			Roots:         tlsConfig.RootCAs,
			CurrentTime:   time.Now(),
			DNSName:       "",
			Intermediates: x509.NewCertPool(),
		}
		for _, cert := range certs[1:] {
			opts.Intermediates.AddCert(cert)
		}

		_, err := certs[0].Verify(opts)
		return err
	}

	return tlsConfig
}

func createConfig() {
	httpclient := &fasthttp.Client{
		MaxIdleConnDuration:           30 * time.Second,
		DisableHeaderNamesNormalizing: true,
		DisablePathNormalizing:        true,
		TLSConfig:                     utils.GetTlsConfig(),
		DialDualStack:                 true,
	}
	config = &utilConfig{
		getToken:  security.GetTokenFunc(),
		m2mTokens: rest.NewM2MTokens(),
		doTimeout: httpclient.DoTimeout,
		client:    httpclient,
	}
}

func getConfig() *utilConfig {
	if config == nil {
		configOnce.Do(createConfig)
	}
	return config
}

func DoRetryRequest(logContext context.Context, method string, url string, data []byte, logger logging.Logger) (*fasthttp.Response, error) {
	attemptDelayStart, _ := strconv.Atoi(configloader.GetOrDefaultString("http.client.retry.attemptDelay", "2000"))
	attemptDelayStartDuration := time.Duration(attemptDelayStart) * time.Millisecond
	retryLimit, _ := strconv.Atoi(configloader.GetOrDefaultString("http.client.retry.maxAttempts", "5"))

	logger.DebugC(logContext, "Execute secure request (retryLimit: %v, retry delay: %v * n)", retryLimit, attemptDelayStart)
	errMsg := ""
	for i := 0; i < retryLimit; i++ {
		if i > 0 {
			waitInterval := attemptDelayStartDuration * time.Duration(i*i)
			logger.InfoC(logContext, "Sleep %v before retry", waitInterval)
			time.Sleep(waitInterval)
		}

		response, err := DoRequest(logContext, method, url, data, logger)
		if err != nil {
			errMsg = fmt.Sprintf("Retrying request %s %s after error:  %s", method, url, err)
			logger.WarnC(logContext, "%s", errMsg)
			continue
		} else {
			return response, nil
		}
	}
	return nil, errors.New(errMsg)
}

// DoRequest sends the request with the M2M token. In hybrid mode a 401 response to the Kubernetes token makes it resend
// the request with the legacy M2M token, and a successful resend makes later requests to url use the legacy token.
func DoRequest(logContext context.Context, method string, url string, data []byte, logger logging.Logger) (*fasthttp.Response, error) {
	tokens := getConfig().m2mTokens
	token, fallbackAllowed, err := tokens.Token(logContext, url)
	if err != nil {
		logger.ErrorC(logContext, "Can't refresh token %v", err)
		errMsg := fmt.Sprintf("Secure %s request handler to %s failed with error: %s", method, url, err)
		logger.WarnC(logContext, "%s", errMsg)
		return nil, errors.New(errMsg)
	}
	response, err := doRequestWithToken(logContext, method, url, data, token, logger)
	if err != nil || !fallbackAllowed || response.StatusCode() != fasthttp.StatusUnauthorized {
		return response, err
	}
	fasthttp.ReleaseResponse(response)
	logger.InfoC(logContext, "%s request to %s was rejected with 401 for the Kubernetes token, resending it with the legacy M2M token", method, url)
	legacyToken, err := tokens.LegacyToken(logContext)
	if err != nil {
		errMsg := fmt.Sprintf("Secure %s request handler to %s failed with error: %s", method, url, err)
		logger.WarnC(logContext, "%s", errMsg)
		return nil, errors.New(errMsg)
	}
	response, err = doRequestWithToken(logContext, method, url, data, legacyToken, logger)
	if err == nil && response.StatusCode() < fasthttp.StatusBadRequest {
		tokens.UseLegacyToken(url)
	}
	return response, err
}

func doRequestWithToken(logContext context.Context, method string, url string, data []byte, token string, logger logging.Logger) (*fasthttp.Response, error) {
	errMsg := ""
	req, err := constructRequest(logContext, method, url, data, token, logger)
	if err != nil {
		fasthttp.ReleaseRequest(req)
		errMsg = fmt.Sprintf("Secure %s request handler to %s failed with error: %s", method, url, err)
		logger.WarnC(logContext, "%s", errMsg)
		return nil, errors.New(errMsg)
	}

	response := fasthttp.AcquireResponse()
	err = getConfig().doTimeout(req, response, 60*time.Second)
	fasthttp.ReleaseRequest(req)
	if err != nil {
		errMsg = fmt.Sprintf("Secure %s request to %s failed with error: %s", method, url, err)
		logger.WarnC(logContext, "%s", errMsg)
		fasthttp.ReleaseResponse(response)
		return nil, errors.New(errMsg)
	}
	if response.StatusCode() >= fasthttp.StatusInternalServerError {
		logger.WarnC(logContext, "Secure %s request to %s failed with 5xx http status code: %d", method, url, response.StatusCode())
		fasthttp.ReleaseResponse(response)
		return nil, errors.New(errMsg)
	} else {
		return response, nil
	}
}

func constructRequest(ctx context.Context, method string, url string, data []byte, m2mToken string, logger logging.Logger) (*fasthttp.Request, error) {
	req := fasthttp.AcquireRequest()
	logger.DebugC(ctx, "Request will be sent with token")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", m2mToken))
	req.Header.Add("Content-Type", "application/json")

	logger.Debugf(`Building secure request with arguments:
	method=%v,
	url=%v`, method, url)

	req.Header.SetMethod(method)
	req.SetRequestURI(url)
	req.SetBody(data)

	if err := ctxhelper.AddSerializableContextData(ctx, req.Header.Set); err != nil {
		logger.ErrorC(ctx, "Error during context serializing: %+v", err)
		return req, err
	}

	return req, nil
}

func SecureWebSocketDial(logContext context.Context, webSocketURL url.URL, dialer websocket.Dialer, requestHeaders http.Header, logger logging.Logger) (*websocket.Conn, *http.Response, error) {
	m2mToken, err := getConfig().getToken(logContext)
	if err != nil {
		logger.ErrorC(logContext, "Can't refresh token %v", err)
		return nil, nil, err
	}
	if requestHeaders == nil {
		logger.WarnC(logContext, "Headers are nil. Creating default headers")
		requestHeaders = http.Header{}
	}
	requestHeaders = addHeaderIfAbsent(requestHeaders, "Host", webSocketURL.Host)
	requestHeaders = addHeaderIfAbsent(requestHeaders, "Authorization", "Bearer "+m2mToken)
	return dialer.Dial(webSocketURL.String(), requestHeaders)
}

func addHeaderIfAbsent(requestHeaders http.Header, headerName, headerValue string) http.Header {
	if _, ok := requestHeaders[headerName]; !ok {
		requestHeaders.Add(headerName, headerValue)
	}
	return requestHeaders
}

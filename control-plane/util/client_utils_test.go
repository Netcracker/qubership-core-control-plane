package util

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"github.com/gorilla/websocket"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-core-lib-go/v3/security"
	"github.com/netcracker/qubership-core-lib-go/v3/security/rest"
	"github.com/netcracker/qubership-core-lib-go/v3/security/tokensource"
	"github.com/netcracker/qubership-core-lib-go/v3/serviceloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

type stubTokenProvider struct {
	security.DummyToken
}

func (p *stubTokenProvider) GetToken(context.Context) (string, error) {
	return "legacy-token", nil
}

type stubTokenSource struct {
	err error
}

func (s *stubTokenSource) GetAudienceToken(context.Context, tokensource.TokenAudience) (string, error) {
	return "k8s-token", s.err
}

func (s *stubTokenSource) GetServiceAccountToken(context.Context) (string, error) {
	return "", nil
}

var k8sTokenSource = &stubTokenSource{}

func TestMain(m *testing.M) {
	serviceloader.Register(1, &security.DummyToken{})
	serviceloader.Register(100, &stubTokenProvider{})
	serviceloader.Register(100, k8sTokenSource)

	configloader.Init()
	os.Exit(m.Run())
}

func TestGetTlsConfigWithoutHostNameValidation(t *testing.T) {
	testTlsConfig := GetTlsConfigWithoutHostNameValidation()
	assert.NotNil(t, testTlsConfig)
	assert.True(t, testTlsConfig.InsecureSkipVerify)
	assert.NotNil(t, testTlsConfig.VerifyPeerCertificate)

	rootCert, intermedCert, userCert := getCerts()
	certBytes := [][]byte{userCert.Bytes, intermedCert.Bytes}

	mockCertificate(getSomeRootCert())
	err := tlsConfig.VerifyPeerCertificate(certBytes, nil)
	assert.NotNil(t, err)
	assert.Equal(t, "x509: certificate signed by unknown authority", err.Error())

	mockCertificate(rootCert)
	err = tlsConfig.VerifyPeerCertificate(certBytes, nil)
	assert.Nil(t, err)

	certBytes = [][]byte{userCert.Bytes} // no Intermediates here
	err = tlsConfig.VerifyPeerCertificate(certBytes, nil)
	assert.NotNil(t, err)
	assert.Equal(t, "x509: certificate signed by unknown authority", err.Error())
}

func mockCertificate(certStr *pem.Block) {
	tlsConfig.RootCAs = x509.NewCertPool()
	testCert, _ := x509.ParseCertificate(certStr.Bytes)
	tlsConfig.RootCAs.AddCert(testCert)
}

func getCerts() (*pem.Block, *pem.Block, *pem.Block) {
	privateKey1, _ := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	certificateTemplate1 := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Root Inc."},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 24),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	certificate1, _ := x509.CreateCertificate(rand.Reader, &certificateTemplate1,
		&certificateTemplate1, &privateKey1.PublicKey, privateKey1)

	privateKey2, _ := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	certificateTemplate2 := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"Intermed Inc."},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 24),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	certificate2, _ := x509.CreateCertificate(rand.Reader, &certificateTemplate2,
		&certificateTemplate1, &privateKey2.PublicKey, privateKey1)

	privateKey3, _ := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	certificateTemplate3 := x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject: pkix.Name{
			Organization: []string{"Monsters Inc."},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour * 24),
		SubjectKeyId: []byte{1, 2, 3, 4, 7},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}

	certificate3, _ := x509.CreateCertificate(rand.Reader, &certificateTemplate3,
		&certificateTemplate2, &privateKey3.PublicKey, privateKey2)

	out1 := &bytes.Buffer{}
	pem.Encode(out1, &pem.Block{Type: "CERTIFICATE", Bytes: certificate1})
	block1, _ := pem.Decode([]byte(out1.String()))

	out2 := &bytes.Buffer{}
	pem.Encode(out2, &pem.Block{Type: "CERTIFICATE", Bytes: certificate2})
	block2, _ := pem.Decode([]byte(out2.String()))

	out3 := &bytes.Buffer{}
	pem.Encode(out3, &pem.Block{Type: "CERTIFICATE", Bytes: certificate3})
	block3, _ := pem.Decode([]byte(out3.String()))

	return block1, block2, block3
}

func getSomeRootCert() *pem.Block {
	privateKey1, _ := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	certificateTemplate1 := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Gremlins Inc."},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 24),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	certificate1, _ := x509.CreateCertificate(rand.Reader, &certificateTemplate1,
		&certificateTemplate1, &privateKey1.PublicKey, privateKey1)

	out1 := &bytes.Buffer{}
	pem.Encode(out1, &pem.Block{Type: "CERTIFICATE", Bytes: certificate1})
	block1, _ := pem.Decode([]byte(out1.String()))

	return block1
}
func TestDoRequestErrOnM2mErr(t *testing.T) {
	useM2MAuthMode(t, security.M2MAuthModeK8s)
	k8sTokenSource.err = errors.New("m2m err")
	t.Cleanup(func() { k8sTokenSource.err = nil })
	resp, err := DoRequest(context.Background(), fasthttp.MethodGet, "http://aaa:8080", nil, logging.GetLogger(""))
	assert.NotNil(t, err)
	assert.Nil(t, resp)
}

func TestConstructRequestFine(t *testing.T) {
	req, err := constructRequest(context.Background(), fasthttp.MethodGet, "http://aaa:8080", nil, "m2m", logging.GetLogger(""))
	assert.Nil(t, err)
	assert.NotNil(t, req)
	assert.Equal(t, "Bearer m2m", string(req.Header.Peek("Authorization")))
	assert.Equal(t, req.Header.Method(), []byte(fasthttp.MethodGet))
	assert.Equal(t, req.RequestURI(), []byte("http://aaa:8080"))
}

func TestDoRetryRequestSecondTryFine(t *testing.T) {
	useM2MAuthMode(t, security.M2MAuthModeLegacy)
	tryNum := 1
	getConfig().doTimeout = func(req *fasthttp.Request, resp *fasthttp.Response, d time.Duration) error {
		if tryNum == 2 {
			resp.SetStatusCode(fasthttp.StatusOK)
			resp.SetBody([]byte("BodyOK"))
			return nil
		}
		tryNum++
		return errors.New("first error on call")
	}

	resp, err := DoRetryRequest(context.Background(), "", "", nil, logging.GetLogger(""))
	assert.Nil(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 2, tryNum)
	assert.Equal(t, fasthttp.StatusOK, resp.StatusCode())
	assert.Equal(t, []byte("BodyOK"), resp.Body())
}

// useM2MAuthMode makes requests send the tokens of mode; the legacy token is "legacy-token" and the Kubernetes token
// is "k8s-token".
func useM2MAuthMode(t *testing.T, mode security.M2MAuthMode) {
	t.Setenv(security.M2MAuthModeEnv, string(mode))
	getConfig().m2mRequestSender = rest.NewM2MRequestSender()
}

// respondInTurn answers each request with the next status in statuses and records the Authorization header it got.
func respondInTurn(statuses ...int) *[]string {
	var gotAuth []string
	getConfig().doTimeout = func(req *fasthttp.Request, resp *fasthttp.Response, d time.Duration) error {
		gotAuth = append(gotAuth, string(req.Header.Peek("Authorization")))
		resp.SetStatusCode(statuses[len(gotAuth)-1])
		return nil
	}
	return &gotAuth
}

func TestDoRequest_SendsTokenAndBody(t *testing.T) {
	useM2MAuthMode(t, security.M2MAuthModeLegacy)
	var gotAuth, gotBody string
	getConfig().doTimeout = func(req *fasthttp.Request, resp *fasthttp.Response, d time.Duration) error {
		gotAuth = string(req.Header.Peek("Authorization"))
		gotBody = string(req.Body())
		resp.SetStatusCode(fasthttp.StatusOK)
		return nil
	}

	resp, err := DoRequest(context.Background(), fasthttp.MethodPost, "http://target:8080/api", []byte("payload"), logging.GetLogger(""))

	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, resp.StatusCode())
	assert.Equal(t, "Bearer legacy-token", gotAuth)
	assert.Equal(t, "payload", gotBody)
}

func TestDoRequest_Returns401WithoutResending(t *testing.T) {
	tests := []struct {
		mode     security.M2MAuthMode
		wantAuth string
	}{
		{mode: security.M2MAuthModeLegacy, wantAuth: "Bearer legacy-token"},
		{mode: security.M2MAuthModeK8s, wantAuth: "Bearer k8s-token"},
	}
	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			useM2MAuthMode(t, tt.mode)
			gotAuth := respondInTurn(fasthttp.StatusUnauthorized)

			resp, err := DoRequest(context.Background(), fasthttp.MethodGet, "http://target:8080/api", nil, logging.GetLogger(""))

			require.NoError(t, err)
			assert.Equal(t, fasthttp.StatusUnauthorized, resp.StatusCode())
			assert.Equal(t, []string{tt.wantAuth}, *gotAuth)
		})
	}
}

func TestDoRequest_HybridResendsWithLegacyTokenAfter401(t *testing.T) {
	useM2MAuthMode(t, security.M2MAuthModeHybrid)
	gotAuth := respondInTurn(fasthttp.StatusUnauthorized, fasthttp.StatusOK)
	var gotBodies []string
	doTimeout := getConfig().doTimeout
	getConfig().doTimeout = func(req *fasthttp.Request, resp *fasthttp.Response, d time.Duration) error {
		gotBodies = append(gotBodies, string(req.Body()))
		return doTimeout(req, resp, d)
	}

	resp, err := DoRequest(context.Background(), fasthttp.MethodPost, "http://target:8080/api", []byte("payload"), logging.GetLogger(""))

	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, resp.StatusCode())
	assert.Equal(t, []string{"Bearer k8s-token", "Bearer legacy-token"}, *gotAuth)
	assert.Equal(t, []string{"payload", "payload"}, gotBodies)
}

func TestDoRequest_HybridKeepsLegacyTokenForTargetAfterSuccessfulResend(t *testing.T) {
	useM2MAuthMode(t, security.M2MAuthModeHybrid)
	gotAuth := respondInTurn(fasthttp.StatusUnauthorized, fasthttp.StatusOK, fasthttp.StatusOK)

	_, err := DoRequest(context.Background(), fasthttp.MethodGet, "http://target:8080/api", nil, logging.GetLogger(""))
	assert.NoError(t, err)
	_, err = DoRequest(context.Background(), fasthttp.MethodGet, "http://target:8080/api", nil, logging.GetLogger(""))

	assert.NoError(t, err)
	assert.Equal(t, []string{"Bearer k8s-token", "Bearer legacy-token", "Bearer legacy-token"}, *gotAuth)
}

func TestDoRequest_HybridDoesNotKeepLegacyTokenAfterFailedResend(t *testing.T) {
	useM2MAuthMode(t, security.M2MAuthModeHybrid)
	gotAuth := respondInTurn(fasthttp.StatusUnauthorized, fasthttp.StatusUnauthorized, fasthttp.StatusOK)

	resp, err := DoRequest(context.Background(), fasthttp.MethodGet, "http://target:8080/api", nil, logging.GetLogger(""))
	require.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, resp.StatusCode())
	_, err = DoRequest(context.Background(), fasthttp.MethodGet, "http://target:8080/api", nil, logging.GetLogger(""))

	assert.NoError(t, err)
	assert.Equal(t, []string{"Bearer k8s-token", "Bearer legacy-token", "Bearer k8s-token"}, *gotAuth)
}

func TestSecureWebSocketDial_HybridRedialsWithLegacyTokenAfter401(t *testing.T) {
	useM2MAuthMode(t, security.M2MAuthModeHybrid)
	var gotAuth []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = append(gotAuth, r.Header.Get("Authorization"))
		if r.Header.Get("Authorization") != "Bearer legacy-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err == nil {
			conn.Close()
		}
	}))
	t.Cleanup(server.Close)
	wsURL, err := url.Parse("ws" + strings.TrimPrefix(server.URL, "http") + "/watch")
	require.NoError(t, err)

	conn, resp, err := SecureWebSocketDial(context.Background(), *wsURL, websocket.Dialer{}, nil, logging.GetLogger(""))

	require.NoError(t, err)
	conn.Close()
	assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	assert.Equal(t, []string{"Bearer k8s-token", "Bearer legacy-token"}, gotAuth)
}

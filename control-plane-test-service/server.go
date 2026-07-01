package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/gofiber/fiber/v3"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

var (
	ctx    = context.Background()
	logger logging.Logger
)

func init() {
	configloader.Init(configloader.BasePropertySources()...)
	logger = logging.GetLogger("main")
}

func main() {
	logger.InfoC(ctx, "Start service...")
	app := fiber.New()
	app.Get("/health", healthHandler)
	app.Get("/certificate", certificateHandler)
	app.Get("/client_certificate", clientCertificateHandler)
	app.Get("/private_key", privateKeyHandler)
	app.All("/api/v1/custom-response-headers", customResponseHeadersHandler)
	app.Post("/api/v1/bus/topics/:topic", busPublishHandler)
	app.All("/*", jsonTraceHandler)

	httpPort := configloader.GetOrDefaultString("http.server.bind", ":8080")
	go func() {
		err := app.Listen(httpPort)
		if err != nil {
			logger.Infof("Error during start http server: %+v", err.Error())
		}
	}()
	// load server certificate
	serverCertificate, err := tls.LoadX509KeyPair("localhost.crt", "localhost.key")
	if err != nil {
		logger.Panic("Cannot load TLS key pair from cert file=%s and key file=%s: %+v", "localhost.crt", "localhost.key", err)
	}

	// Read client cert file
	clientCertificates, err := ioutil.ReadFile("localhostclient.crt")
	if err != nil {
		logger.Panic("Failed to read certificate from file=%s due to error=%+v", "localhostclient.crt", err)
	}

	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	// Append certificate to root CA
	if ok := rootCAs.AppendCertsFromPEM(clientCertificates); !ok {
		logger.Panic("No clientCertificates appended to trust store")
	}

	tlsConfig := &tls.Config{
		RootCAs:    rootCAs,
		MinVersion: tls.VersionTLS12,
		ClientAuth: tls.VerifyClientCertIfGiven,
		ClientCAs:  rootCAs,
		Certificates: []tls.Certificate{
			serverCertificate,
		},
		CurvePreferences: getEcdhCurves(),
	}

	httpsBind := os.Getenv("HTTPS_SERVER_BIND")
	if httpsBind == "" {
		httpsBind = ":8443"
	}

	server := &http.Server{
		Addr:      httpsBind,
		TLSConfig: tlsConfig,
		Handler: http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			fmt.Fprint(res, "OK")
		}),
	}

	logger.Info("Start https server on %s", httpsBind)
	logger.Panic("can not start server: %v", server.ListenAndServeTLS("localhost.crt", "localhost.key"))
}

func getEcdhCurves() []tls.CurveID {
	ecdhCurvesEnvValue := os.Getenv("ECDH_CURVES")
	logger.Infof("Found ECDH_CURVES: %s", ecdhCurvesEnvValue)
	if ecdhCurvesEnvValue == "" {
		logger.Infof("Default ecdh curves")
		return nil
	}

	ecdhCurves := make([]tls.CurveID, 0)
	for _, ecdh := range strings.Split(ecdhCurvesEnvValue, ",") {
		switch value := ecdh; value {
		case "P-256":
			ecdhCurves = append(ecdhCurves, tls.CurveP256)
		case "P-384":
			ecdhCurves = append(ecdhCurves, tls.CurveP384)
		case "P-521":
			ecdhCurves = append(ecdhCurves, tls.CurveP521)
		default:
			logger.Panic("Unknown ecdh curves: %s", value)
		}
	}

	logger.Infof("Current ecdh curves: %s", ecdhCurves)
	return ecdhCurves
}

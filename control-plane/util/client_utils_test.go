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
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-core-lib-go/v3/security"
	"github.com/netcracker/qubership-core-lib-go/v3/serviceloader"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	"math/big"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	serviceloader.Register(1, &security.DummyToken{})

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
func TestConstructRequestErrOnM2mErr(t *testing.T) {
	getConfig().getToken = func(context.Context) (string, error) {
		return "", errors.New("m2m err")
	}
	req, err := constructRequest(context.Background(), fasthttp.MethodGet, "http://aaa:8080", nil, logging.GetLogger(""))
	assert.NotNil(t, err)
	assert.NotNil(t, req)
}

func TestConstructRequestFine(t *testing.T) {
	getConfig().getToken = func(context.Context) (string, error) {
		return "m2m", nil
	}
	req, err := constructRequest(context.Background(), fasthttp.MethodGet, "http://aaa:8080", nil, logging.GetLogger(""))
	assert.Nil(t, err)
	assert.NotNil(t, req)
	assert.Equal(t, "Bearer m2m", string(req.Header.Peek("Authorization")))
	assert.Equal(t, req.Header.Method(), []byte(fasthttp.MethodGet))
	assert.Equal(t, req.RequestURI(), []byte("http://aaa:8080"))
}

func TestDoRetryRequestSecondTryFine(t *testing.T) {
	getConfig().getToken = func(context.Context) (string, error) {
		return "m2m", nil
	}
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

func TestDoRequest_SendsTokenAndBody(t *testing.T) {
	getConfig().getToken = func(context.Context) (string, error) {
		return "m2m", nil
	}
	var gotAuth, gotBody string
	getConfig().doTimeout = func(req *fasthttp.Request, resp *fasthttp.Response, d time.Duration) error {
		gotAuth = string(req.Header.Peek("Authorization"))
		gotBody = string(req.Body())
		resp.SetStatusCode(fasthttp.StatusOK)
		return nil
	}

	resp, err := DoRequest(context.Background(), fasthttp.MethodPost, "http://target:8080/api", []byte("payload"), logging.GetLogger(""))

	assert.NoError(t, err)
	assert.Equal(t, fasthttp.StatusOK, resp.StatusCode())
	assert.Equal(t, "Bearer m2m", gotAuth)
	assert.Equal(t, "payload", gotBody)
}

func TestDoRequest_Returns401WithoutResending(t *testing.T) {
	getConfig().getToken = func(context.Context) (string, error) {
		return "m2m", nil
	}
	calls := 0
	getConfig().doTimeout = func(req *fasthttp.Request, resp *fasthttp.Response, d time.Duration) error {
		calls++
		resp.SetStatusCode(fasthttp.StatusUnauthorized)
		return nil
	}

	resp, err := DoRequest(context.Background(), fasthttp.MethodGet, "http://target:8080/api", nil, logging.GetLogger(""))

	assert.NoError(t, err)
	assert.Equal(t, fasthttp.StatusUnauthorized, resp.StatusCode())
	assert.Equal(t, 1, calls)
}

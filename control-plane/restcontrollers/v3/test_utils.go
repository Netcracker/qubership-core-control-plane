package v3

import (
	"bytes"
	"github.com/gofiber/fiber/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	fiberserver "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v3"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"testing"
)

func saveDeploymentVersions(t *testing.T, storage *dao.InMemDao, dVs ...*domain.DeploymentVersion) {
	_, err := storage.WithWTx(func(dao dao.Repository) error {
		for _, dV := range dVs {
			assert.Nil(t, dao.SaveDeploymentVersion(dV))
		}
		return nil
	})
	assert.Nil(t, err)
}

func SendHttpRequestWithoutBody(t *testing.T, httpMethod, endpoint, reqUrl string, f func(ctx fiber.Ctx) error) *http.Response {
	return SendHttpRequestWithBody(t, httpMethod, endpoint, reqUrl, bytes.NewBufferString(""), f)
}

func SendHttpRequestWithBody(t *testing.T, httpMethod, endpoint, reqUrl string, body io.Reader, f func(ctx fiber.Ctx) error) *http.Response {
	fiberConfig := fiber.Config{
		ErrorHandler: errorcodes.DefaultErrorHandlerWrapper(errorcodes.UnknownErrorCode),
	}
	app, err := fiberserver.New(fiberConfig).Process()
	assert.Nil(t, err)
	app.Add([]string{httpMethod}, endpoint, f)

	req, err := http.NewRequest(httpMethod,
		reqUrl,
		body,
	)
	req.Host = "localhost"
	assert.Nil(t, err)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	assert.Nil(t, err)
	return resp
}

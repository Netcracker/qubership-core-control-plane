package v2

import (
	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	fiberserver "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v2"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"testing"
)

func SaveDeploymentVersions(t *testing.T, storage *dao.InMemDao, dVs ...*domain.DeploymentVersion) {
	_, err := storage.WithWTx(func(dao dao.Repository) error {
		for _, dV := range dVs {
			assert.Nil(t, dao.SaveDeploymentVersion(dV))
		}
		return nil
	})
	assert.Nil(t, err)
}

func SendHttpRequestWithBody(t *testing.T, httpMethod, endpoint, reqUrl string, body io.Reader, f func(fiberCtx *fiber.Ctx) error) *http.Response {
	app, err := fiberserver.New().Process()
	assert.Nil(t, err)
	app.Add(httpMethod, endpoint, f)
	req, err := http.NewRequest(httpMethod,
		reqUrl,
		body,
	)
	assert.Nil(t, err)
	resp, err := app.Test(req, -1)
	return resp
}

package v2

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	mock_dao "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/dao"
	fiberserver "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlueGreenController_deleteVersionResponses(t *testing.T) {
	tests := []struct {
		name            string
		found           *domain.DeploymentVersion
		err             error
		wantStatus      int
		wantContentType string
		wantBody        string
	}{
		{name: "lookup fails", err: errors.New("lookup failed"), wantStatus: http.StatusInternalServerError,
			wantContentType: "application/json", wantBody: `{"error":"lookup failed"}`},
		{name: "version not found", wantStatus: http.StatusNotFound,
			wantContentType: "text/plain; charset=utf-8", wantBody: "404 Page not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := mock_dao.NewMockDao(gomock.NewController(t))
			data.EXPECT().FindDeploymentVersion("v2").Return(tt.found, tt.err)
			app, err := fiberserver.New(fiber.Config{ErrorHandler: errorcodes.DefaultErrorHandlerWrapper(errorcodes.UnknownErrorCode)}).Process()
			require.NoError(t, err)
			app.Delete("/versions/:version", NewBlueGreenController(nil, data).HandleDeleteDeploymentVersionWithID)

			resp, err := app.Test(httptest.NewRequest(http.MethodDelete, "/versions/v2", nil), -1)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, tt.wantStatus, resp.StatusCode, "status")
			assert.Equal(t, tt.wantContentType, resp.Header.Get("Content-Type"), "Content-Type")
			assert.Equal(t, tt.wantBody, string(body), "body")
		})
	}
}

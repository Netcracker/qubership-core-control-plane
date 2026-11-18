package v3

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	fiberserver "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBGRegistryController_readsFormFieldsOfEveryEncoding(t *testing.T) {
	registryController := NewBGRegistryController(nil, nil)
	app := newErrorHandlingApp(t)
	app.Get("/registry", registryController.HandleGetMicroserviceVersions)
	app.Post("/registry", registryController.HandleGetMicroserviceVersions)

	var multipartBody bytes.Buffer
	writer := multipart.NewWriter(&multipartBody)
	require.NoError(t, writer.WriteField("initialVersion", "v1"))
	require.NoError(t, writer.Close())

	tests := []struct {
		name        string
		method      string
		target      string
		contentType string
		body        string
	}{
		{name: "query string", method: http.MethodGet, target: "/registry?initialVersion=v1"},
		{name: "urlencoded body", method: http.MethodPost, target: "/registry", contentType: "application/x-www-form-urlencoded", body: "initialVersion=v1"},
		{name: "multipart body", method: http.MethodPost, target: "/registry", contentType: writer.FormDataContentType(), body: multipartBody.String()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.target, strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			status, contentType, body := sendTestRequest(t, app, req)

			assert.Equal(t, http.StatusBadRequest, status, "status, body %s", body)
			assert.Equal(t, "application/json", contentType, "Content-Type")
			assert.Contains(t, body, "initialVersion param cannot be specified without serviceName", "body")
		})
	}
}

func TestRoutingModeController_detailsAreJSON(t *testing.T) {
	app := newErrorHandlingApp(t)
	app.Get("/details", controller.HandleGetRoutingModeDetails)

	status, contentType, body := sendTestRequest(t, app, httptest.NewRequest(http.MethodGet, "/details", nil))

	assert.Equal(t, http.StatusOK, status, "status, body %s", body)
	assert.Equal(t, "application/json", contentType, "Content-Type")
}

func newErrorHandlingApp(t *testing.T) *fiber.App {
	t.Helper()
	app, err := fiberserver.New(fiber.Config{ErrorHandler: errorcodes.DefaultErrorHandlerWrapper(errorcodes.UnknownErrorCode)}).Process()
	require.NoError(t, err)
	return app
}

func sendTestRequest(t *testing.T, app *fiber.App, req *http.Request) (int, string, string) {
	t.Helper()
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, resp.Header.Get("Content-Type"), string(body)
}

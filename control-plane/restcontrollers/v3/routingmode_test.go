package v3

import (
	"bytes"
	"encoding/json"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/ram"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/routingmode"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
	fiberserver "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v3"
	security2 "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v3/security"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/serviceloader"
	"github.com/stretchr/testify/assert"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
)

var (
	service    *routingmode.Service
	controller *RoutingModeController
	genericDao *dao.InMemDao
)

func TestMain(m *testing.M) {
	serviceloader.Register(1, &security2.DummyFiberServerSecurityMiddleware{})

	inMemStorage := ram.NewStorage()
	genericDao = dao.NewInMemDao(inMemStorage, &idGeneratorMock{}, nil)
	service = routingmode.NewService(genericDao, "v1")
	controller = NewRoutingModeController(service)
	configloader.Init(configloader.EnvPropertySource())
	os.Exit(m.Run())
}

func TestAllowedRoutesMiddlewareV1(t *testing.T) {
	app, err := fiberserver.New().Process()
	if err != nil {
		log.Errorf("Error during app creation")
		t.Fatal("Error during app creation", http.StatusBadRequest)
	}
	app.Use(controller.AllowedRoutesMiddlewareV1())
	app.Get("/test", func(ctx fiber.Ctx) error {
		return ctx.SendStatus(http.StatusOK)
	})

	routes := make([]dto.RouteEntry, 1)
	routes[0] = dto.RouteEntry{
		Namespace: "NS",
	}
	req := dto.RouteEntityRequest{
		Routes: &routes,
	}
	jsonPayload, _ := json.Marshal(req)
	_, code, _ := sendReq(app, http.MethodGet, "/test", string(jsonPayload))
	if code != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v",
			code, http.StatusOK)
	}

	service.UpdateRoutingMode("v2", msaddr.NewNamespace("nNS"))
	_, code, _ = sendReq(app, http.MethodGet, "/test", string(jsonPayload))
	if code != http.StatusBadRequest {
		t.Fatalf("handler returned wrong status code: got %v want %v",
			code, http.StatusBadRequest)
	}
}

func TestAllowedRoutesMiddlewareV3(t *testing.T) {
	app, err := fiberserver.New().Process()
	if err != nil {
		log.Errorf("Error during app creation")
		t.Fatal("Error during app creation", http.StatusBadRequest)
	}
	app.Use(controller.ValidateRoutesApplicabilityToCurrentRoutingMode())
	app.Get("/test", func(ctx fiber.Ctx) error {
		return ctx.SendStatus(http.StatusOK)
	})

	req := []dto.RouteRegistrationRequest{{
		Version:   "v1",
		Namespace: "default",
	}}
	jsonPayload, _ := json.Marshal(req)
	_, code, _ := sendReq(app, http.MethodGet, "/test", string(jsonPayload))
	if code != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v",
			code, http.StatusOK)
	}

	req = []dto.RouteRegistrationRequest{{
		Version:   "v2",
		Namespace: "my-test-ns",
	}}
	jsonPayload, _ = json.Marshal(req)
	_, code, _ = sendReq(app, http.MethodGet, "/test", string(jsonPayload))
	if code != http.StatusBadRequest {
		t.Fatalf("handler returned wrong status code: got %v want %v",
			code, http.StatusBadRequest)
	}

	service.UpdateRoutingMode("v2", msaddr.NewNamespace("nNS"))
	req = []dto.RouteRegistrationRequest{{
		Version:   "v2",
		Namespace: "my-test-ns",
	}}
	jsonPayload, _ = json.Marshal(req)
	_, code, _ = sendReq(app, http.MethodGet, "/test", string(jsonPayload))
	if code != http.StatusBadRequest {
		t.Fatalf("handler returned wrong status code: got %v want %v",
			code, http.StatusBadRequest)
	}

	service.SetRoutingMode(routingmode.SIMPLE)
	service.UpdateRoutingMode("", msaddr.NewNamespace("new-ns"))
	req = []dto.RouteRegistrationRequest{{
		Version: "v3",
	}}
	jsonPayload, _ = json.Marshal(req)
	_, code, _ = sendReq(app, http.MethodGet, "/test", string(jsonPayload))
	if code != http.StatusBadRequest {
		t.Fatalf("handler returned wrong status code: got %v want %v",
			code, http.StatusBadRequest)
	}
}

func TestRoutingModeService_UpdateRoutingMode(t *testing.T) {
	//routingmode.SetDefaultDeployVersion("v1")
	var routingModeTestSet = []struct {
		version  string
		ns       string
		expected routingmode.RoutingMode
	}{
		{"", "", routingmode.SIMPLE},
		{"v1", "", routingmode.SIMPLE},
		{"v1", "default", routingmode.SIMPLE},
		{"", "default", routingmode.SIMPLE},
		{"v2", "default", routingmode.VERSIONED},
		{"v2", "", routingmode.VERSIONED},
		{"v2", "ns", routingmode.VERSIONED},
		{"v1", "ns", routingmode.NAMESPACED},
		{"", "ns", routingmode.NAMESPACED},
	}
	for _, tt := range routingModeTestSet {
		t.Run("{"+tt.version+"|"+tt.ns+"}", func(t *testing.T) {
			service.SetRoutingMode(routingmode.SIMPLE) //reset to default
			service.UpdateRoutingMode(tt.version, msaddr.NewNamespace(tt.ns))
			rm := service.GetRoutingMode()
			if rm != tt.expected {
				t.Errorf("got %s, want %s", rm, tt.expected)
			}
		})
	}
}

func TestRoutingModeService_HandleGetRoutingModeDetails(t *testing.T) {
	app, err := fiberserver.New().Process()
	if err != nil {
		log.Errorf("Error during app creation")
		t.Fatal("Error during app creation", http.StatusBadRequest)
	}
	app.Get("/test", func(ctx fiber.Ctx) error {
		return controller.HandleGetRoutingModeDetails(ctx)
	})

	var routingModeTestSet = []struct {
		version  string
		ns       string
		expected routingmode.RoutingMode
	}{
		{"", "", routingmode.SIMPLE},
		{"v1", "", routingmode.SIMPLE},
		{"v1", msaddr.CurrentNamespaceAsString(), routingmode.SIMPLE},
		{"", msaddr.CurrentNamespaceAsString(), routingmode.SIMPLE},
		{"v2", msaddr.CurrentNamespaceAsString(), routingmode.VERSIONED},
		{"v2", "", routingmode.VERSIONED},
		{"v2", msaddr.CurrentNamespaceAsString() + "ns", routingmode.MIXED},
		{"v1", msaddr.CurrentNamespaceAsString() + "ns", routingmode.NAMESPACED},
		{"", msaddr.CurrentNamespaceAsString() + "ns", routingmode.NAMESPACED},
	}
	for _, tt := range routingModeTestSet {
		t.Run("{"+tt.version+"|"+tt.ns+"}", func(t *testing.T) {
			dpVersion := "v1"
			if tt.version != "" {
				dpVersion = tt.version
			}
			route := &domain.Route{
				Id:                       1,
				Uuid:                     uuid.NewString(),
				RouteKey:                 "key1",
				DeploymentVersion:        dpVersion,
				InitialDeploymentVersion: dpVersion,
			}
			headerMatcher := &domain.HeaderMatcher{
				Id:         int32(1),
				RouteId:    int32(1),
				Name:       "namespace",
				ExactMatch: tt.ns,
			}
			deploymentVersion := &domain.DeploymentVersion{Version: "v1", Stage: domain.ActiveStage}
			deploymentVersionSecond := &domain.DeploymentVersion{Version: "v2", Stage: domain.CandidateStage}

			_, err = genericDao.WithWTx(func(dao dao.Repository) error {
				if tt.version != "" || tt.ns != "" {
					assert.Nil(t, dao.SaveRoute(route))
				}
				if tt.ns != "" {
					assert.Nil(t, dao.SaveHeaderMatcher(headerMatcher))
				}
				if tt.version != "" {
					assert.Nil(t, dao.SaveDeploymentVersion(deploymentVersion))
					assert.Nil(t, dao.SaveDeploymentVersion(deploymentVersionSecond))
				}

				return nil
			})
			if err != nil {
				panic(err)
			}

			summary := sendReqForRoutingModeDetails(t, app)
			if summary.RoutingMode != tt.expected {
				t.Errorf("got %s, want %s", summary.RoutingMode, tt.expected)
			}

			_, err = genericDao.WithWTx(func(dao dao.Repository) error {
				if tt.version != "" || tt.ns != "" {
					assert.Nil(t, dao.DeleteRouteByUUID(route.Uuid))
				}
				if tt.ns != "" {
					assert.Nil(t, dao.DeleteHeaderMatcher(headerMatcher))
				}
				if tt.version != "" {
					assert.Nil(t, dao.DeleteDeploymentVersions([]*domain.DeploymentVersion{deploymentVersion, deploymentVersionSecond}))
				}
				return nil
			})
			if err != nil {
				panic(err)
			}
		})
	}
}

func TestRoutingModeService_IsForbiddenRoutingMode(t *testing.T) {
	//routingmode.SetDefaultDeployVersion("v1")
	var isForbiddenRoutingModeTestSet = []struct {
		routingMode routingmode.RoutingMode
		version     string
		ns          string
		expected    bool
	}{
		{routingmode.SIMPLE, "", "", false},
		{routingmode.SIMPLE, "", "default", false},
		{routingmode.SIMPLE, "v1", "", false},
		{routingmode.SIMPLE, "v1", "default", false},
		{routingmode.SIMPLE, "", "ns", false},
		{routingmode.SIMPLE, "v2", "", false},
		{routingmode.SIMPLE, "v2", "ns", false},
		{routingmode.SIMPLE, "v2", "default", false},

		{routingmode.NAMESPACED, "", "", false},
		{routingmode.NAMESPACED, "", "default", false},
		{routingmode.NAMESPACED, "v1", "", false},
		{routingmode.NAMESPACED, "v1", "default", false},
		{routingmode.NAMESPACED, "", "ns", false},
		{routingmode.NAMESPACED, "v2", "", true},
		{routingmode.NAMESPACED, "v2", "ns", true},
		{routingmode.NAMESPACED, "v2", "default", true},

		{routingmode.VERSIONED, "", "", false},
		{routingmode.VERSIONED, "", "default", false},
		{routingmode.VERSIONED, "v1", "", false},
		{routingmode.VERSIONED, "v1", "default", false},
		{routingmode.VERSIONED, "", "ns", true},
		{routingmode.VERSIONED, "v2", "", false},
		{routingmode.VERSIONED, "v2", "ns", true},
		{routingmode.VERSIONED, "v2", "default", false},
	}
	for _, tt := range isForbiddenRoutingModeTestSet {
		t.Run("{"+tt.routingMode.String()+"|"+tt.version+"|"+tt.ns+"}", func(t *testing.T) {
			service.SetRoutingMode(tt.routingMode)
			actual := service.IsForbiddenRoutingMode(tt.version, tt.ns)
			if actual != tt.expected {
				t.Errorf("got %t, want %t", actual, tt.expected)
			}
		})
	}
}

func sendReq(app *fiber.App, method, path, payload string) (string, int, error) {
	req, err := http.NewRequest(method, path, bytes.NewBufferString(payload))
	req.Host = "localhost"
	if err != nil {
		return "", 0, err
	}

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	bodyBytes, _ := io.ReadAll(resp.Body)

	return string(bodyBytes), resp.StatusCode, err
}

func sendReqForRoutingModeDetails(t *testing.T, app *fiber.App) routingmode.Summary {
	req, err := http.NewRequest(http.MethodGet, "/test", nil)
	req.Host = "localhost"
	if err != nil {
		assert.FailNow(t, err.Error())
	}

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	defer resp.Body.Close()
	bodyBytes, readErr := ioutil.ReadAll(resp.Body)
	assert.Nil(t, readErr)
	var summary routingmode.Summary
	err = json.Unmarshal(bodyBytes, &summary)
	assert.Nil(t, err)
	assert.NotEmpty(t, summary)

	return summary
}

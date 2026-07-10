package v3

import (
	"bytes"
	"encoding/json"
	"github.com/gofiber/fiber/v3"
	"github.com/hashicorp/go-memdb"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/event/bus"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/ram"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/active"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"strings"
	"testing"
)

const (
	ActiveActiveConfigPostPath = "/api/v3/control-plane/active-active"

	TestPublicGwHostDc1 = "test.public.dc-1.gw"
	TestPublicGwHostDc2 = "test.public.dc-2.gw"
	TestPublicGwHostDc3 = "test.public.dc-3.gw"

	TestPrivateGwHostDc1 = "test.private.dc-1.gw"
	TestPrivateGwHostDc2 = "test.private.dc-2.gw"
	TestPrivateGwHostDc3 = "test.private.dc-3.gw"

	LocalPublicGwHost = TestPublicGwHostDc1
	LocalRivateGwHost = TestPrivateGwHostDc1
)

func TestActiveDCsController_HandleActiveActiveConfigPost(t *testing.T) {
	testDataSlice := []map[string]interface{}{
		{
			"protocol":       "https",
			"publicGwHosts":  []string{TestPublicGwHostDc1, TestPublicGwHostDc2, TestPublicGwHostDc3},
			"privateGwHosts": []string{TestPrivateGwHostDc1, TestPrivateGwHostDc2, TestPrivateGwHostDc3},
			"statusCode":     http.StatusOK,
		},
		{
			"protocol":       "http",
			"publicGwHosts":  []string{TestPublicGwHostDc1, TestPublicGwHostDc2, TestPublicGwHostDc3},
			"privateGwHosts": []string{TestPrivateGwHostDc1, TestPrivateGwHostDc2, TestPrivateGwHostDc3},
			"statusCode":     http.StatusOK,
		},
		{
			"protocol":       "ftp",
			"publicGwHosts":  []string{TestPublicGwHostDc1, TestPublicGwHostDc2, TestPublicGwHostDc3},
			"privateGwHosts": []string{TestPrivateGwHostDc1, TestPrivateGwHostDc2, TestPrivateGwHostDc3},
			"statusCode":     http.StatusBadRequest,
			"errMsgContains": "'protocol' can be either http or https",
		},
		{
			"protocol":       "",
			"publicGwHosts":  []string{TestPublicGwHostDc1, TestPublicGwHostDc2, TestPublicGwHostDc3},
			"privateGwHosts": []string{TestPrivateGwHostDc1, TestPrivateGwHostDc2, TestPrivateGwHostDc3},
			"statusCode":     http.StatusBadRequest,
			"errMsgContains": "'protocol' cannot be empty",
		},
		{
			"protocol":       "https",
			"publicGwHosts":  []string{TestPublicGwHostDc1, TestPublicGwHostDc2, TestPublicGwHostDc3},
			"privateGwHosts": []string{},
			"statusCode":     http.StatusBadRequest,
			"errMsgContains": "'publicGwHosts' or 'privateGwHosts' cannot be empty",
		},
		{
			"protocol":       "https",
			"publicGwHosts":  []string{},
			"privateGwHosts": []string{TestPrivateGwHostDc1, TestPrivateGwHostDc2, TestPrivateGwHostDc3},
			"statusCode":     http.StatusBadRequest,
			"errMsgContains": "'publicGwHosts' or 'privateGwHosts' cannot be empty",
		},
		{
			"protocol":       "https",
			"publicGwHosts":  []string{},
			"privateGwHosts": []string{},
			"statusCode":     http.StatusBadRequest,
			"errMsgContains": "'publicGwHosts' or 'privateGwHosts' cannot be empty",
		},
		{
			"protocol":       "https",
			"publicGwHosts":  []string{TestPublicGwHostDc1, TestPublicGwHostDc2},
			"privateGwHosts": []string{TestPrivateGwHostDc1, TestPrivateGwHostDc2, TestPrivateGwHostDc3},
			"statusCode":     http.StatusBadRequest,
			"errMsgContains": "'publicGwHosts' and 'privateGwHosts' must contain the same amount of elements",
		},
		{
			"protocol":       "https",
			"publicGwHosts":  []string{"", ""},
			"privateGwHosts": []string{"", ""},
			"statusCode":     http.StatusBadRequest,
			"errMsgContains": "'publicGwHosts' and 'privateGwHosts' cannot contain empty elements",
		},
		{
			"protocol":       "https",
			"publicGwHosts":  []string{TestPublicGwHostDc1 + ":8080", TestPublicGwHostDc2 + ":8080"},
			"privateGwHosts": []string{TestPrivateGwHostDc1 + ":8080", TestPrivateGwHostDc2 + ":8080"},
			"statusCode":     http.StatusBadRequest,
			"errMsgContains": "host elements from 'publicGwHosts' and 'privateGwHosts' cannot contain ':'",
		},
	}
	for _, testData := range testDataSlice {
		activeDCsController := getActiveDCsController(t)
		requestAsMap := map[string]interface{}{
			"protocol":       testData["protocol"],
			"publicGwHosts":  testData["publicGwHosts"],
			"privateGwHosts": testData["privateGwHosts"],
		}
		requestAsBytes, err := json.Marshal(requestAsMap)
		assert.Nil(t, err)
		response := SendHttpRequestWithBody(t, http.MethodPost, ActiveActiveConfigPostPath, ActiveActiveConfigPostPath,
			bytes.NewBuffer(requestAsBytes), activeDCsController.HandleActiveActiveConfigPostUnsecure)
		assert.Equal(t, testData["statusCode"].(int), response.StatusCode)
		if errMsgContains, ok := testData["errMsgContains"]; ok {
			responseBodyBytes, _ := io.ReadAll(response.Body)
			responseBodyStr := string(responseBodyBytes)
			assert.NotEmpty(t, responseBodyStr)
			assert.True(t, strings.Contains(responseBodyStr, errMsgContains.(string)))
		}
	}
}

func TestActiveDCsController_HandleActiveActiveConfigDelete(t *testing.T) {
	activeDCsController := getActiveDCsController(t)
	requestAsMap := map[string]interface{}{
		"protocol":       "https",
		"publicGwHosts":  []string{TestPublicGwHostDc1, TestPublicGwHostDc2, TestPublicGwHostDc3},
		"privateGwHosts": []string{TestPrivateGwHostDc1, TestPrivateGwHostDc2, TestPrivateGwHostDc3},
	}
	requestAsBytes, err := json.Marshal(requestAsMap)
	assert.Nil(t, err)
	responseRecorder := SendHttpRequestWithBody(t, http.MethodPost, ActiveActiveConfigPostPath, ActiveActiveConfigPostPath,
		bytes.NewBuffer(requestAsBytes), activeDCsController.HandleActiveActiveConfigPostUnsecure)
	assert.Equal(t, http.StatusOK, responseRecorder.StatusCode)

	// delete active-active config
	responseRecorder = SendHttpRequestWithBody(t, http.MethodDelete, ActiveActiveConfigPostPath, ActiveActiveConfigPostPath,
		bytes.NewBuffer(nil), activeDCsController.HandleActiveActiveConfigDeleteUnsecure)
	assert.Equal(t, http.StatusOK, responseRecorder.StatusCode)

	// additional delete active-active config attempt must be successful as well
	responseRecorder = SendHttpRequestWithBody(t, http.MethodDelete, ActiveActiveConfigPostPath, ActiveActiveConfigPostPath,
		bytes.NewBuffer(nil), activeDCsController.HandleActiveActiveConfigDeleteUnsecure)
	assert.Equal(t, http.StatusOK, responseRecorder.StatusCode)
}

func (c *ActiveDCsController) HandleActiveActiveConfigPostUnsecure(ctx fiber.Ctx) error {
	return c.HandleActiveActiveConfigPost(ctx)
}

func (c *ActiveDCsController) HandleActiveActiveConfigDeleteUnsecure(ctx fiber.Ctx) error {
	return c.HandleActiveActiveConfigDelete(ctx)
}

func getActiveDCsController(t *testing.T) *ActiveDCsController {
	inMemStorage := ram.NewStorage()
	internalBus := bus.GetInternalBusInstance()
	eventBus := bus.NewEventBusAggregator(inMemStorage, internalBus, internalBus, nil, nil)
	genericDao := dao.NewInMemDao(inMemStorage, &idGeneratorMock{}, []func([]memdb.Change) error{flushChanges})
	entityService := entity.NewService("v1")
	activeDCsService := active.NewActiveDCsService(genericDao, entityService, eventBus, LocalPublicGwHost, LocalRivateGwHost)
	lbController := NewActiveDCsController(activeDCsService)
	v1 := domain.NewDeploymentVersion("v1", domain.ActiveStage)
	saveDeploymentVersions(t, genericDao, v1)
	return lbController
}

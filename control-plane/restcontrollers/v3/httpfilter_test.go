package v3

import (
	"bytes"
	"encoding/json"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/dto"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/cluster"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/entity"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/httpFilter"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/httpFilter/extAuthz"
	mock_dao "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/dao"
	mock_bus "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/event/bus"
	mock_extAuthz "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/services/httpFilter/extAuthz"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"testing"
)

func TestHttpFilterController_HandlePostExtAuthz(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := mock_dao.NewMockDao(ctrl)
	bus := mock_bus.NewMockBusPublisher(ctrl)
	entitySrv := entity.NewService("v1")
	clusterSrv := cluster.NewClusterService(entitySrv, mockDao, bus)
	extAuthzService := mock_extAuthz.NewMockService(ctrl)
	srv := httpFilter.NewWasmFilterService(mockDao, bus, clusterSrv, entitySrv, extAuthzService)

	controller := NewHttpFilterController(srv)

	req := `brokenJson`
	response := SendHttpRequestWithBody(t, http.MethodPost, "/api/v3/http-filters", "/api/v3/http-filters",
		bytes.NewBufferString(req), controller.HandlePostHttpFilters)
	assert.Equal(t, http.StatusBadRequest, response.StatusCode)

	req = `{
                "extAuthzFilter": {
                    "contextExtensions": {
                        "key1": "val1"
                    },
                    "destination": {
                        "cluster": "ext-authz",
                        "endpoint": "gateway-auth-exension:10050"
                    },
                    "name": "integrationExtAuthzFilter"
                },
                "gateways": [
                    "integration-gateway"
                ]
            }`
	extAuthzDto := dto.ExtAuthz{
		Name: "integrationExtAuthzFilter",
		Destination: dto.RouteDestination{
			Cluster:  "ext-authz",
			Endpoint: "gateway-auth-exension:10050",
		},
		ContextExtensions: map[string]string{"key1": "val1"},
	}

	extAuthzService.EXPECT().ValidateApply(gomock.Any(), extAuthzDto, "integration-gateway").
		Return(false, "expected validation err in test")

	response = SendHttpRequestWithBody(t, http.MethodPost, "/api/v3/http-filters", "/api/v3/http-filters",
		bytes.NewBufferString(req), controller.HandlePostHttpFilters)
	assert.Equal(t, http.StatusBadRequest, response.StatusCode)

	extAuthzService.EXPECT().ValidateApply(gomock.Any(), extAuthzDto, "integration-gateway").
		Return(true, "")
	extAuthzService.EXPECT().Apply(gomock.Any(), extAuthzDto, "integration-gateway").Return(extAuthz.ErrNameTaken)

	response = SendHttpRequestWithBody(t, http.MethodPost, "/api/v3/http-filters", "/api/v3/http-filters",
		bytes.NewBufferString(req), controller.HandlePostHttpFilters)
	assert.Equal(t, http.StatusBadRequest, response.StatusCode)

	extAuthzService.EXPECT().ValidateApply(gomock.Any(), extAuthzDto, "integration-gateway").
		Return(true, "")
	extAuthzService.EXPECT().Apply(gomock.Any(), extAuthzDto, "integration-gateway").Return(errors.New("some internal err"))

	response = SendHttpRequestWithBody(t, http.MethodPost, "/api/v3/http-filters", "/api/v3/http-filters",
		bytes.NewBufferString(req), controller.HandlePostHttpFilters)
	assert.Equal(t, http.StatusInternalServerError, response.StatusCode)

	extAuthzService.EXPECT().ValidateApply(gomock.Any(), extAuthzDto, "integration-gateway").
		Return(true, "")
	extAuthzService.EXPECT().Apply(gomock.Any(), extAuthzDto, "integration-gateway").Return(nil)

	response = SendHttpRequestWithBody(t, http.MethodPost, "/api/v3/http-filters", "/api/v3/http-filters",
		bytes.NewBufferString(req), controller.HandlePostHttpFilters)
	assert.Equal(t, http.StatusOK, response.StatusCode)
}

func TestHttpFilterController_HandleDeleteExtAuthz(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := mock_dao.NewMockDao(ctrl)
	bus := mock_bus.NewMockBusPublisher(ctrl)
	entitySrv := entity.NewService("v1")
	clusterSrv := cluster.NewClusterService(entitySrv, mockDao, bus)
	extAuthzService := mock_extAuthz.NewMockService(ctrl)
	srv := httpFilter.NewWasmFilterService(mockDao, bus, clusterSrv, entitySrv, extAuthzService)

	controller := NewHttpFilterController(srv)

	req := `brokenJson`
	response := SendHttpRequestWithBody(t, http.MethodDelete, "/api/v3/http-filters", "/api/v3/http-filters",
		bytes.NewBufferString(req), controller.HandleDeleteHttpFilters)
	assert.Equal(t, http.StatusBadRequest, response.StatusCode)

	req = `{
                "extAuthzFilter": {
                    "name": "integrationExtAuthzFilter"
                },
                "gateways": [
                    "integration-gateway"
                ]
            }`
	extAuthzDto := dto.ExtAuthz{Name: "integrationExtAuthzFilter"}

	extAuthzService.EXPECT().ValidateDelete(gomock.Any(), extAuthzDto, "integration-gateway").
		Return(false, "expected validation err in test")

	response = SendHttpRequestWithBody(t, http.MethodDelete, "/api/v3/http-filters", "/api/v3/http-filters",
		bytes.NewBufferString(req), controller.HandleDeleteHttpFilters)
	assert.Equal(t, http.StatusBadRequest, response.StatusCode)

	extAuthzService.EXPECT().ValidateDelete(gomock.Any(), extAuthzDto, "integration-gateway").
		Return(true, "")
	extAuthzService.EXPECT().Delete(gomock.Any(), extAuthzDto, "integration-gateway").Return(extAuthz.ErrNameTaken)

	response = SendHttpRequestWithBody(t, http.MethodDelete, "/api/v3/http-filters", "/api/v3/http-filters",
		bytes.NewBufferString(req), controller.HandleDeleteHttpFilters)
	assert.Equal(t, http.StatusBadRequest, response.StatusCode)

	extAuthzService.EXPECT().ValidateDelete(gomock.Any(), extAuthzDto, "integration-gateway").
		Return(true, "")
	extAuthzService.EXPECT().Delete(gomock.Any(), extAuthzDto, "integration-gateway").Return(errors.New("some internal err"))

	response = SendHttpRequestWithBody(t, http.MethodDelete, "/api/v3/http-filters", "/api/v3/http-filters",
		bytes.NewBufferString(req), controller.HandleDeleteHttpFilters)
	assert.Equal(t, http.StatusInternalServerError, response.StatusCode)

	extAuthzService.EXPECT().ValidateDelete(gomock.Any(), extAuthzDto, "integration-gateway").
		Return(true, "")
	extAuthzService.EXPECT().Delete(gomock.Any(), extAuthzDto, "integration-gateway").Return(nil)

	response = SendHttpRequestWithBody(t, http.MethodDelete, "/api/v3/http-filters", "/api/v3/http-filters",
		bytes.NewBufferString(req), controller.HandleDeleteHttpFilters)
	assert.Equal(t, http.StatusOK, response.StatusCode)
}

func TestHttpFilterController_HandleGetHttpFilters(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDao := mock_dao.NewMockDao(ctrl)
	bus := mock_bus.NewMockBusPublisher(ctrl)
	entitySrv := entity.NewService("v1")
	clusterSrv := cluster.NewClusterService(entitySrv, mockDao, bus)
	extAuthzService := mock_extAuthz.NewMockService(ctrl)
	srv := httpFilter.NewWasmFilterService(mockDao, bus, clusterSrv, entitySrv, extAuthzService)

	controller := NewHttpFilterController(srv)

	mockDao.EXPECT().FindListenersByNodeGroupId("integration-gateway").Return(nil, errors.New("some db err"))
	response := SendHttpRequestWithBody(t, http.MethodGet, "/api/v3/http-filters/:nodeGroup", "/api/v3/http-filters/integration-gateway",
		bytes.NewBufferString(""), controller.HandleGetHttpFilters)
	assert.Equal(t, http.StatusInternalServerError, response.StatusCode)

	mockDao.EXPECT().FindListenersByNodeGroupId("integration-gateway").Return([]*domain.Listener{{
		Id:                     1,
		Name:                   "integration-gateway-listener",
		BindHost:               "0.0.0.0",
		BindPort:               "8080",
		RouteConfigurationName: "integration-gateway-routes",
		Version:                1,
		NodeGroupId:            "integration-gateway",
	}}, nil)

	mockDao.EXPECT().FindWasmFilterByListenerId(int32(1)).Return([]*domain.WasmFilter{
		{
			Id:     1,
			Name:   "testWasmFilter1",
			URL:    "test-url1",
			SHA256: "test-sha1",
		},
		{
			Id:     2,
			Name:   "testWasmFilter2",
			URL:    "test-url2",
			SHA256: "test-sha2",
		}}, nil)
	extAuthzService.EXPECT().Get(gomock.Any(), "integration-gateway").Return(&dto.ExtAuthz{
		Name:              "testExtAuthz",
		Destination:       dto.RouteDestination{Cluster: "test-cluster", Endpoint: "test-endpoint:8080"},
		ContextExtensions: map[string]string{"key1": "val1"},
	}, nil)

	response = SendHttpRequestWithBody(t, http.MethodGet, "/api/v3/http-filters/:nodeGroup", "/api/v3/http-filters/integration-gateway",
		bytes.NewBufferString(""), controller.HandleGetHttpFilters)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	bodeBytes, err := ioutil.ReadAll(response.Body)
	assert.Nil(t, err)
	var actual dto.HttpFiltersConfigRequestV3
	assert.Nil(t, json.Unmarshal(bodeBytes, &actual))

	assert.Equal(t, 2, len(actual.WasmFilters))
	assert.Equal(t, dto.ExtAuthz{
		Name:              "testExtAuthz",
		Destination:       dto.RouteDestination{Cluster: "test-cluster", Endpoint: "test-endpoint:8080"},
		ContextExtensions: map[string]string{"key1": "val1"},
	}, *actual.ExtAuthzFilter)
}

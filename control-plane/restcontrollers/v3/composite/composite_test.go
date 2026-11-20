package composite

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/composite"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/errorcodes"
	v3 "github.com/netcracker/qubership-core-control-plane/control-plane/v2/restcontrollers/v3"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/services/routingmode"
	mock_dao "github.com/netcracker/qubership-core-control-plane/control-plane/v2/test/mock/dao"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/util/msaddr"
	"github.com/netcracker/qubership-core-lib-go-error-handling/v3/tmf"
	security2 "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v3/security"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/serviceloader"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
)

var (
	service    *routingmode.Service
	controller *v3.RoutingModeController
)

const (
	testGetCompositeNamespaces  = "/api/v3/composite-platform/namespaces"
	testPostCompositeNamespaces = "/api/v3/composite-platform/namespaces/:namespace"
)

func TestHandleRemoveNamespaceFromComposite_shouldReturnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	controller, _ := getV3Controller(ctrl)

	// ValidationRequestError - empty namespace
	response := v3.SendHttpRequestWithBody(t, http.MethodGet,
		testPostCompositeNamespaces,
		strings.Replace(testPostCompositeNamespaces, ":namespace", " ", 1),
		bytes.NewBuffer(nil),
		controller.HandleRemoveNamespaceFromComposite,
	)
	assert.NotNil(t, response)
	assert.Equal(t, errorcodes.ValidationRequestError.GetHttpCode(), response.StatusCode)
	tmfResponse := readTmfResponse(t, response)
	assert.Equal(t, errorcodes.ValidationRequestError.Code, tmfResponse.Code)

	// CompositeConflictError
	response = v3.SendHttpRequestWithBody(t, http.MethodGet,
		testPostCompositeNamespaces,
		strings.Replace(testPostCompositeNamespaces, ":namespace", msaddr.DefaultNamespace, 1),
		bytes.NewBuffer(nil),
		controller.HandleRemoveNamespaceFromComposite,
	)
	assert.NotNil(t, response)
	assert.Equal(t, errorcodes.CompositeConflictError.GetHttpCode(), response.StatusCode)
	tmfResponse = readTmfResponse(t, response)
	assert.Equal(t, errorcodes.CompositeConflictError.Code, tmfResponse.Code)

	// UnknownErrorCode
	response = v3.SendHttpRequestWithBody(t, http.MethodGet,
		testPostCompositeNamespaces,
		strings.Replace(testPostCompositeNamespaces, ":namespace", "test", 1),
		bytes.NewBuffer(nil),
		controller.HandleRemoveNamespaceFromComposite,
	)
	assert.NotNil(t, response)
	assert.Equal(t, 500, response.StatusCode)
	tmfResponse = readTmfResponse(t, response)
	assert.Equal(t, errorcodes.UnknownErrorCode.Code, tmfResponse.Code)
}

func TestHandleAddNamespaceToComposite_shouldReturnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	controller, _ := getV3Controller(ctrl)

	// ValidationRequestError - empty namespace
	response := v3.SendHttpRequestWithBody(t, http.MethodGet,
		testPostCompositeNamespaces,
		strings.Replace(testPostCompositeNamespaces, ":namespace", " ", 1),
		bytes.NewBuffer(nil),
		controller.HandleAddNamespaceToComposite,
	)
	assert.NotNil(t, response)
	assert.Equal(t, errorcodes.ValidationRequestError.GetHttpCode(), response.StatusCode)
	tmfResponse := readTmfResponse(t, response)
	assert.Equal(t, errorcodes.ValidationRequestError.Code, tmfResponse.Code)

	// CompositeConflictError
	response = v3.SendHttpRequestWithBody(t, http.MethodGet,
		testPostCompositeNamespaces,
		strings.Replace(testPostCompositeNamespaces, ":namespace", msaddr.DefaultNamespace, 1),
		bytes.NewBuffer(nil),
		controller.HandleAddNamespaceToComposite,
	)
	assert.NotNil(t, response)
	assert.Equal(t, errorcodes.CompositeConflictError.GetHttpCode(), response.StatusCode)
	tmfResponse = readTmfResponse(t, response)
	assert.Equal(t, errorcodes.CompositeConflictError.Code, tmfResponse.Code)

	// UnknownErrorCode
	response = v3.SendHttpRequestWithBody(t, http.MethodGet,
		testPostCompositeNamespaces,
		strings.Replace(testPostCompositeNamespaces, ":namespace", "test", 1),
		bytes.NewBuffer(nil),
		controller.HandleAddNamespaceToComposite,
	)
	assert.NotNil(t, response)
	assert.Equal(t, 500, response.StatusCode)
	tmfResponse = readTmfResponse(t, response)
	assert.Equal(t, errorcodes.UnknownErrorCode.Code, tmfResponse.Code)
}

func TestHandleGetCompositeStructure_shouldReturnError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	controller, mockDao := getV3Controller(ctrl)

	// UnknownErrorCode
	mockDao.EXPECT().WithRTxVal(gomock.Any()).AnyTimes().Return(nil, fmt.Errorf("test error"))
	response := v3.SendHttpRequestWithoutBody(t, http.MethodGet, testGetCompositeNamespaces, testGetCompositeNamespaces, controller.HandleGetCompositeStructure)
	assert.NotNil(t, response)
	assert.Equal(t, 500, response.StatusCode)
	tmfResponse := readTmfResponse(t, response)
	assert.Equal(t, errorcodes.UnknownErrorCode.Code, tmfResponse.Code)
}

func TestMain(m *testing.M) {
	serviceloader.Register(1, &security2.DummyFiberServerSecurityMiddleware{})
	configloader.Init(configloader.EnvPropertySource())
	os.Exit(m.Run())
}

func getV3Controller(ctrl *gomock.Controller) (*Controller, *mock_dao.MockDao) {
	mockDao := getErrorMockDao(ctrl)
	service := getService(mockDao)
	return NewCompositeController(service), mockDao
}

func getService(mockDao *mock_dao.MockDao) *composite.Service {
	configloader.Init(configloader.EnvPropertySource())
	return composite.NewService("", composite.BaselineMode, mockDao, nil, nil, nil)
}

func getErrorMockDao(ctrl *gomock.Controller) *mock_dao.MockDao {
	mockDao := mock_dao.NewMockDao(ctrl)
	mockDao.EXPECT().WithWTx(gomock.Any()).AnyTimes().Return(nil, fmt.Errorf("test error"))
	return mockDao
}

func readTmfResponse(t *testing.T, response *http.Response) tmf.Response {
	tmfResponse := tmf.Response{}
	body, err := ioutil.ReadAll(response.Body)
	assert.Nil(t, err)
	err = json.Unmarshal(body, &tmfResponse)

	return tmfResponse
}

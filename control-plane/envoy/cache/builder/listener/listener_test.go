package listener

import (
	"fmt"
	listenerV3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	managerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/ptypes"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/domain"
	"github.com/netcracker/qubership-core-control-plane/control-plane/v2/envoy/cache/builder/common"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGatewayListenerBuilder_BuildListener(t *testing.T) {
	configloader.Init(configloader.EnvPropertySource())

	builder := NewGatewayListenerBuilder(common.NewEnvoyProxyProperties())
	testListenerBuilding(t, builder, true, false)
}

func TestFacadeListenerBuilder_BuildListener(t *testing.T) {
	configloader.Init(configloader.EnvPropertySource())

	builder := NewFacadeListenerBuilder(&common.TracingProperties{})
	testListenerBuilding(t, builder, false, false)
}

func TestGatewayListenerBuilder_BuildListenerWithTls(t *testing.T) {
	configloader.Init(configloader.EnvPropertySource())

	builder := NewGatewayListenerBuilder(common.NewEnvoyProxyProperties())
	testListenerBuilding(t, builder, true, true)
}

func TestFacadeListenerBuilder_BuildListenerWithTls(t *testing.T) {
	configloader.Init(configloader.EnvPropertySource())

	builder := NewFacadeListenerBuilder(&common.TracingProperties{})
	testListenerBuilding(t, builder, false, true)
}

func testListenerBuilding(t *testing.T, builder ListenerBuilder, checkCors bool, withTls bool) {
	bindPort := uint32(8080)
	if withTls {
		bindPort = uint32(8443)
	}
	domainListener := &domain.Listener{
		Id:                     1,
		Name:                   "test-listener",
		BindHost:               "0.0.0.0",
		BindPort:               fmt.Sprintf("%v", bindPort),
		RouteConfigurationName: "test-listener-routes",
		Version:                1,
		NodeGroupId:            "test-gateway",
		NodeGroup:              nil,
		WasmFilters:            nil,
	}

	listener, err := builder.BuildListener(domainListener, "", withTls)
	assert.Nil(t, err)

	if withTls {
		assert.Equal(t, "test-listener-tls", listener.Name)
	} else {
		assert.Equal(t, "test-listener", listener.Name)
	}

	socketAddr := listener.GetAddress().GetSocketAddress()
	assert.Equal(t, "0.0.0.0", socketAddr.Address)
	assert.Equal(t, bindPort, socketAddr.GetPortValue())
	assert.False(t, socketAddr.GetIpv4Compat())

	if withTls {
		assert.Equal(t, wellknown.TransportSocketTls, listener.FilterChains[0].TransportSocket.Name)
		assert.NotNil(t, listener.FilterChains[0].TransportSocket.GetTypedConfig())
		assert.Equal(t, wellknown.TlsInspector, listener.ListenerFilters[0].Name)
		assert.NotNil(t, listener.ListenerFilters[0].GetTypedConfig())
	} else {
		assert.Nil(t, listener.FilterChains[0].TransportSocket)
		assert.Nil(t, listener.ListenerFilters)
	}

	httpConnManager := getHttpConnManager(t, listener)
	if checkCors {
		verifyHttpConnManagerFilter(t, httpConnManager, wellknown.CORS, false)
	}
	verifyHttpConnManagerFilter(t, httpConnManager, wellknown.Router, true)
}

func getHttpConnManager(t *testing.T, listener *listenerV3.Listener) *managerv3.HttpConnectionManager {
	httpConnManagerFilter := listener.GetFilterChains()[0].Filters[0]
	assert.Equal(t, wellknown.HTTPConnectionManager, httpConnManagerFilter.Name)
	httpConnManagerBytes := httpConnManagerFilter.GetTypedConfig()
	assert.NotNil(t, httpConnManagerBytes)
	httpConnManager := &managerv3.HttpConnectionManager{}
	assert.Nil(t, ptypes.UnmarshalAny(httpConnManagerBytes, httpConnManager))
	return httpConnManager
}

func verifyHttpConnManagerFilter(t *testing.T, httpConnManager *managerv3.HttpConnectionManager, filterName string, mustBeTheLast bool) {
	if mustBeTheLast {
		filter := httpConnManager.GetHttpFilters()[len(httpConnManager.GetHttpFilters())-1]
		assert.Equal(t, filterName, filter.GetName())
		assert.NotNil(t, filter.GetTypedConfig())
	} else {
		for _, filter := range httpConnManager.GetHttpFilters() {
			if filter.Name == filterName {
				assert.NotNil(t, filter.GetTypedConfig())
				return
			}
		}
		t.Errorf("Expected filter %s was not found in HttpConnectionManager#HttpFilters", filterName)
	}
}

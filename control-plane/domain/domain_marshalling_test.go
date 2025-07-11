package domain

import (
	"bytes"
	"encoding/gob"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNodeGroupMarshal_Success(t *testing.T) {
	nodeGroup := NodeGroup{Name: "A"}
	cluster := Cluster{Id: 1, NodeGroups: []*NodeGroup{&nodeGroup}}
	nodeGroup.Clusters = []*Cluster{&cluster}
	AssertMarshalSuccessful(t, &nodeGroup)
}

func TestListenerMarshal_Success(t *testing.T) {
	listener := Listener{}
	wf := WasmFilter{}
	wf.Listeners = []Listener{listener}
	listener.WasmFilters = []WasmFilter{wf}

	lf := LuaFilter{}
	lf.Listeners = []Listener{listener}
	listener.LuaFilters = []LuaFilter{lf}
	AssertMarshalSuccessful(t, &listener)
}

func TestClusterMarshal_Success(t *testing.T) {

	ng := NodeGroup{}
	nodes := []*NodeGroup{&ng}

	cluster := Cluster{}
	cluster.NodeGroups = nodes

	ng.Clusters = []*Cluster{&cluster}

	tls := TlsConfig{NodeGroups: nodes}
	cluster.TLS = &tls

	endpoint := Endpoint{Cluster: &cluster}
	endpoints := []*Endpoint{&endpoint}
	cluster.Endpoints = endpoints

	hcheck := HealthCheck{Cluster: &cluster}
	hchecks := []*HealthCheck{&hcheck}
	cluster.HealthChecks = hchecks

	AssertMarshalSuccessful(t, &cluster)
}

func TestTlsConfigMarshal_Success(t *testing.T) {

	ng := NodeGroup{}
	nodes := []*NodeGroup{&ng}

	cluster := Cluster{}
	cluster.NodeGroups = nodes

	ng.Clusters = []*Cluster{&cluster}

	tls := TlsConfig{NodeGroups: nodes}
	cluster.TLS = &tls

	AssertMarshalSuccessful(t, &tls)
}

func TestCircuitBreakerMarshal_Success(t *testing.T) {
	circuitBreaker := CircuitBreaker{}
	threshold := &Threshold{Id: 1, MaxConnections: 2}
	circuitBreaker.Threshold = threshold
	circuitBreaker.ThresholdId = threshold.Id
	AssertMarshalSuccessful(t, &circuitBreaker)
}

func TestThresholdMarshal_Success(t *testing.T) {
	threshold := &Threshold{Id: 1, MaxConnections: 2}
	AssertMarshalSuccessful(t, threshold)
}

func TestClustersNodeGroupMarshal_Success(t *testing.T) {
	cng := ClustersNodeGroup{}
	cng.Cluster = &Cluster{}
	cng.NodeGroup = &NodeGroup{}
	assert.Nil(t, cng.MarshalPrepare())
	assert.Nil(t, cng.Cluster)
	assert.Nil(t, cng.NodeGroup)
}

func TestListenersWasmFilter(t *testing.T) {
	lwf := ListenersWasmFilter{}
	lwf.Listener = &Listener{}
	lwf.WasmFilter = &WasmFilter{}
	assert.Nil(t, lwf.MarshalPrepare())
	assert.Nil(t, lwf.Listener)
	assert.Nil(t, lwf.WasmFilter)
}

func TestEndpointMarshal_Success(t *testing.T) {
	endpoint := Endpoint{}
	endpoint.Cluster = &Cluster{}
	endpoint.Cluster.Endpoints = []*Endpoint{&endpoint}
	AssertMarshalSuccessful(t, &endpoint)
}

func TestRouteConfigurationMarshal_Success(t *testing.T) {
	rc := RouteConfiguration{}
	vh := VirtualHost{}
	vh.RouteConfiguration = &rc
	rc.VirtualHosts = []*VirtualHost{&vh}
	AssertMarshalSuccessful(t, &rc)
}

func TestVirtualHostMarshal_Success(t *testing.T) {
	vh := VirtualHost{}
	rc := RouteConfiguration{}
	rc.VirtualHosts = []*VirtualHost{&vh}
	vh.RouteConfiguration = &rc
	AssertMarshalSuccessful(t, &vh)
}

func TestVirtualHostDomainMarshal_Success(t *testing.T) {
	vhd := VirtualHostDomain{}
	vh := VirtualHost{}
	vh.Domains = []*VirtualHostDomain{&vhd}
	vhd.VirtualHost = &vh
	AssertMarshalSuccessful(t, &vhd)
}

func TestRouteMarshal_Success(t *testing.T) {
	route := Route{}
	vh := VirtualHost{}
	vh.Routes = []*Route{&route}
	route.VirtualHost = &vh
	AssertMarshalSuccessful(t, &route)
}

func TestHeaderMatcherMarshal_Success(t *testing.T) {
	hm := HeaderMatcher{}
	route := Route{}
	route.HeaderMatchers = []*HeaderMatcher{&hm}
	hm.Route = &route
	AssertMarshalSuccessful(t, &hm)
}

func TestHashPolicyMarshal_Success(t *testing.T) {
	hp := HashPolicy{}
	r := Route{}
	r.HashPolicies = []*HashPolicy{&hp}
	hp.Route = &r
	AssertMarshalSuccessful(t, &hp)
}

func TestRetryPolicyMarshal_Success(t *testing.T) {
	rp := RetryPolicy{}
	r := Route{}
	r.RetryPolicy = &rp
	rp.Route = &r
	AssertMarshalSuccessful(t, &rp)
}

func TestHealthCheckMarshal_Success(t *testing.T) {
	hc := HealthCheck{}
	httpHC := HttpHealthCheck{}
	httpHC.HealthCheck = &hc
	hc.HttpHealthCheck = &httpHC
	AssertMarshalSuccessful(t, &hc)
}

func TestWasmFilterMarshal_Success(t *testing.T) {
	wf := WasmFilter{}
	l := Listener{}
	l.WasmFilters = []WasmFilter{wf}
	wf.Listeners = []Listener{l}
	AssertMarshalSuccessful(t, &wf)
}

func TestStatefulSessionMarshal_Success(t *testing.T) {
	ses := StatefulSession{}
	dv := DeploymentVersion{}
	ses.DeploymentVersionVal = &dv
	AssertMarshalSuccessful(t, &ses)
}

func TestMicroserviceVersionMarshal_Success(t *testing.T) {
	msVer := MicroserviceVersion{}
	dv := DeploymentVersion{}
	msVer.DeploymentVersionVal = &dv
	AssertMarshalSuccessful(t, &msVer)
}

func AssertMarshalSuccessful(t *testing.T, preparer MarshalPreparer) {
	buf := bytes.Buffer{}
	err := preparer.MarshalPrepare()
	assert.Nil(t, err)
	err = gob.NewEncoder(&buf).Encode(preparer)
	assert.Nil(t, err)
	assert.True(t, len(buf.Bytes()) > 0)
}

func TestLuaFilterMarshal_Success(t *testing.T) {
	lf := LuaFilter{}
	l := Listener{}
	l.LuaFilters = []LuaFilter{lf}
	lf.Listeners = []Listener{l}
	AssertMarshalSuccessful(t, &lf)
}

func TestListenersLuaFilter(t *testing.T) {
	llf := ListenersLuaFilter{}
	llf.Listener = &Listener{}
	llf.LuaFilter = &LuaFilter{}
	assert.Nil(t, llf.MarshalPrepare())
	assert.Nil(t, llf.Listener)
	assert.Nil(t, llf.LuaFilter)
}

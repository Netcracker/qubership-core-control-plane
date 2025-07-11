package domain

// MarshalPreparer is used for preparing instance to serialization process. Domain entities can contain recursive
// links, so we have to remove them to correctly perform serialization
type MarshalPreparer interface {
	MarshalPrepare() error
}

func (ng *NodeGroup) MarshalPrepare() error {
	ng.Clusters = nil
	return nil
}

func (l *Listener) MarshalPrepare() error {
	l.NodeGroup = nil
	l.WasmFilters = nil
	l.LuaFilters =nil
	l.ExtAuthzFilter = nil
	return nil
}

func (c *Cluster) MarshalPrepare() error {
	c.NodeGroups = nil
	c.Endpoints = nil
	c.HealthChecks = nil
	c.CircuitBreaker = nil
	c.TLS = nil
	return nil
}

func (m *MicroserviceVersion) MarshalPrepare() error {
	m.DeploymentVersionVal = nil
	return nil
}

func (c *TlsConfig) MarshalPrepare() error {
	c.NodeGroups = nil
	return nil
}

func (c *TlsConfigsNodeGroups) MarshalPrepare() error {
	c.NodeGroup = nil
	c.TlsConfig = nil
	return nil
}

func (cng *ClustersNodeGroup) MarshalPrepare() error {
	cng.Cluster = nil
	cng.NodeGroup = nil
	return nil
}

func (lwf *ListenersWasmFilter) MarshalPrepare() error {
	lwf.Listener = nil
	lwf.WasmFilter = nil
	return nil
}

func (e *Endpoint) MarshalPrepare() error {
	e.Cluster = nil
	e.HashPolicies = nil
	e.StatefulSession = nil
	return nil
}

func (rc *RouteConfiguration) MarshalPrepare() error {
	rc.NodeGroup = nil
	rc.VirtualHosts = nil
	return nil
}

func (_ *DeploymentVersion) MarshalPrepare() error {
	return nil
}

func (vh *VirtualHost) MarshalPrepare() error {
	vh.RouteConfiguration = nil
	vh.Routes = nil
	vh.Domains = nil
	vh.RateLimit = nil
	return nil
}

func (vhd *VirtualHostDomain) MarshalPrepare() error {
	vhd.VirtualHost = nil
	return nil
}

func (r *Route) MarshalPrepare() error {
	r.VirtualHost = nil
	r.HeaderMatchers = nil
	r.HashPolicies = nil
	r.RetryPolicy = nil
	r.StatefulSession = nil
	r.RateLimit = nil
	return nil
}

func (h *HeaderMatcher) MarshalPrepare() error {
	h.Route = nil
	return nil
}

func (h *HashPolicy) MarshalPrepare() error {
	h.Route = nil
	h.Endpoint = nil
	return nil
}

func (r *RetryPolicy) MarshalPrepare() error {
	r.Route = nil
	return nil
}

func (hc *HealthCheck) MarshalPrepare() error {
	hc.HttpHealthCheck = nil
	hc.TlsOptions = nil
	hc.Cluster = nil
	return nil
}

func (w *WasmFilter) MarshalPrepare() error {
	w.Listeners = nil
	return nil
}

func (_ *EnvoyConfigVersion) MarshalPrepare() error {
	return nil
}

func (_ *CompositeSatellite) MarshalPrepare() error {
	return nil
}

func (s *StatefulSession) MarshalPrepare() error {
	s.DeploymentVersionVal = nil
	return nil
}

func (_ *RateLimit) MarshalPrepare() error {
	return nil
}

func (_ *ExtAuthzFilter) MarshalPrepare() error {
	return nil
}

func (cb *CircuitBreaker) MarshalPrepare() error {
	cb.Threshold = nil
	return nil
}

func (_ *Threshold) MarshalPrepare() error {
	return nil
}

func (_ *TcpKeepalive) MarshalPrepare() error {
	return nil
}

func (w *LuaFilter) MarshalPrepare() error {
	w.Listeners = nil
	return nil
}

func (llf *ListenersLuaFilter) MarshalPrepare() error {
	llf.Listener = nil
	llf.LuaFilter = nil
	return nil
}
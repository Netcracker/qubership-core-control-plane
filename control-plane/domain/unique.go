package domain

type Unique interface {
	TableName() string
	GetId() int32
	SetId(id int32)
}

func (c *Cluster) TableName() string {
	return ClusterTable
}

func (c *Cluster) GetId() int32 {
	return c.Id
}

func (c *Cluster) SetId(id int32) {
	c.Id = id
}

func (r *Route) TableName() string {
	return RouteTable
}

func (r *Route) GetId() int32 {
	return r.Id
}

func (r *Route) SetId(id int32) {
	r.Id = id
}

func (h *HeaderMatcher) TableName() string {
	return HeaderMatcherTable
}

func (h *HeaderMatcher) GetId() int32 {
	return h.Id
}

func (h *HeaderMatcher) SetId(id int32) {
	h.Id = id
}

func (vh *VirtualHost) TableName() string {
	return VirtualHostTable
}

func (vh *VirtualHost) GetId() int32 {
	return vh.Id
}

func (vh *VirtualHost) SetId(id int32) {
	vh.Id = id
}

func (rc *RouteConfiguration) TableName() string {
	return RouteConfigurationTable
}

func (rc *RouteConfiguration) GetId() int32 {
	return rc.Id
}

func (rc *RouteConfiguration) SetId(id int32) {
	rc.Id = id
}

func (e *Endpoint) TableName() string {
	return EndpointTable
}

func (e *Endpoint) GetId() int32 {
	return e.Id
}

func (e *Endpoint) SetId(id int32) {
	e.Id = id
}

func (l *Listener) TableName() string {
	return ListenerTable
}

func (l *Listener) GetId() int32 {
	return l.Id
}

func (l *Listener) SetId(id int32) {
	l.Id = id
}

func (h *HashPolicy) TableName() string {
	return HashPolicyTable
}

func (h *HashPolicy) GetId() int32 {
	return h.Id
}

func (h *HashPolicy) SetId(id int32) {
	h.Id = id
}

func (r *RetryPolicy) TableName() string {
	return RetryPolicyTable
}

func (r *RetryPolicy) GetId() int32 {
	return r.Id
}

func (r *RetryPolicy) SetId(id int32) {
	r.Id = id
}

func (hc *HealthCheck) TableName() string {
	return HealthCheckTable
}

func (hc *HealthCheck) GetId() int32 {
	return hc.Id
}

func (hc *HealthCheck) SetId(id int32) {
	hc.Id = id
}

func (r *TlsConfig) TableName() string {
	return TlsConfigTable
}

func (r *TlsConfig) GetId() int32 {
	return r.Id
}

func (r *TlsConfig) SetId(id int32) {
	r.Id = id
}

func (w *WasmFilter) TableName() string {
	return WasmFilterTable
}

func (w *WasmFilter) GetId() int32 {
	return w.Id
}

func (w *WasmFilter) SetId(id int32) {
	w.Id = id
}

func (l *LuaFilter) TableName() string {
	return LuaFilterTable
}

func (l *LuaFilter) GetId() int32 {
	return l.Id
}

func (l *LuaFilter) SetId(id int32) {
	l.Id = id
}

func (s *StatefulSession) TableName() string {
	return StatefulSessionTable
}

func (s *StatefulSession) GetId() int32 {
	return s.Id
}

func (s *StatefulSession) SetId(id int32) {
	s.Id = id
}

func (s *CircuitBreaker) TableName() string {
	return CircuitBreakerTable
}

func (s *CircuitBreaker) GetId() int32 {
	return s.Id
}

func (s *CircuitBreaker) SetId(id int32) {
	s.Id = id
}

func (s *Threshold) TableName() string {
	return ThresholdTable
}

func (s *Threshold) GetId() int32 {
	return s.Id
}

func (s *Threshold) SetId(id int32) {
	s.Id = id
}

func (s *NodeGroup) TableName() string {
	return NodeGroupTable
}

func (ng *NodeGroup) GetId() int32 {
	return runtimeIdByNameGen.GetIdByName(ng.Name)
}

func (s *NodeGroup) SetId(id int32) {
	// is not applicable for NodeGroup
}

func (s *TcpKeepalive) TableName() string {
	return TcpKeepaliveTable
}

func (s *TcpKeepalive) GetId() int32 {
	return s.Id
}

func (s *TcpKeepalive) SetId(id int32) {
	s.Id = id
}

## RouteConfiguration to HTTPRoute
Source:

    apiVersion: core.netcracker.com/v1
    kind: Mesh
    subKind: RouteConfiguration  ← must be present to identify as RouteConfiguration

Target:

    apiVersion: gateway.networking.k8s.io/v1
    kind: HTTPRoute

Input fields → Output fields:

    metadata:

        name: <n>       → used in HTTPRoute.metadata.name. Refer to `HTTPRoute name resolution`
        labels: { ... } → refer to common label resolution rules

    spec:

        namespace         string             OMIT
        gateways          []string           → spec.parentRefs          refer to parentRef resolution
        listenerPort      int                → spec.parentRef[].port   
        tlsSupported      bool               OMIT
        virtualServices   []VirtualService   → one HTTPRoute per entry
        overridden        bool               OMIT

### HTTPRoute name resolution

  Single virtualService in Mesh CR:
    
    HTTPRoute.metadata.name = Mesh CR metadata.name

  Multiple virtualServices in Mesh CR:
    
    HTTPRoute.metadata.name = Mesh CR metadata.name + "-" + virtualService.name

###  RouteConfiguration.spec.gateways to HTTPRoute.spec.parentRefs resolution (priority order)

Source field: 
    
    RouteConfiguration.spec.gateways

Target field: 

    HTTProute.spec.parentRefs

Mapping:

  PRIORITY 1 — Platform gateway table:

    parentRef type: Gateway
    mapping: one-to-one 
    condition: gateway is in list of platform Gateways
    parentRef name resolution:
        source name              parentRef name
        public-gateway-service  → public-gateway
        private-gateway-service → private-gateway
        egress-gateway          → egress-gateway

        kind: omit
        group: omit
        By default parent assumed to be Gateway

Example:
```yaml
spec: 
    parentRefs:
    - name: <platform Gateway name, e.g. public-gateway>
```

  PRIORITY 2 — ingress/egress gateway:

    parentRef type: Gateway
    mapping: one-to-one 
    condition: gateway is in list of discovered ingress/egress Gateways
    parentRef name resolution:
        name = ingress/egress Gateway name

        kind: omit
        group: omit
        name: <gateway metadata.name value>

Example:
```yaml
spec: 
    parentRefs:
    - name: <ingress/egress Gateway name>
```

  PRIORITY 3 — Internal gateway or mesh Gateway:

    parentRef type: Service
    mapping: one-to-many (one parentRef per host entry)
    condition: gateway = `internal-gateway-service` OR gateway is in list of discovered mesh Gateways
    parentRef name resolution:
        normalized host from virtualService.hosts[]

Example:
```yaml
spec:
    parentRefs:
    - kind: Service
    group: ''
    name: <normalized host from virtualService.hosts[0]>
    - kind: Service
    group: ''
    name: <normalized host from virtualService.hosts[1]>
    ...        
```
---

### VirtualService

  JSON key            Go type              Transformation
  ────────────────────────────────────────────────────────────────────────────────
  name                string               → HTTPRoute name suffix when multiple VSes exist
  hosts               []string             → parentRef Service names (mesh routes) + spec.hostnames[]
                                             normalize each
  rateLimit           string               OMIT  ⚠ flag for manual review if non-empty
  addHeaders          []HeaderDefinition   → RequestHeaderModifier filter add[] (virtualService-level)
  removeHeaders       []string             → RequestHeaderModifier filter remove[] (virtualService-level)
  routeConfiguration  RouteConfig          → HTTPRoute rules[]
  overridden          bool                 OMIT


### HTTPRoute.spec.hostnames resolution

```yaml
spec:
    hostnames:
    - <normalized host 0>
    - <normalized host 1>
    ...
```

#### Host normalization
  IF host = `*` - ignore it, do not propagate to spec.hostnames[]. If `*` occurs in east-west Route - manual review required
  ELSE IF host contains ".":
    result = host.split(".")[0]
    e.g. "my-svc.namespace"    → "my-svc"
    e.g. "my-svc.ns:8080"     → "my-svc"
  ELSE:
    result = host unchanged
    e.g. "{{ .Values.SERVICE }}" → "{{ .Values.SERVICE }}"

---

### RouteConfig

  JSON key  Go type     Transformation
  ──────────────────────────────────────────────────
  version   string      OMIT
  routes    []RouteV3   → flatten all rules into HTTPRoute rules[]

---

### RouteV3

  JSON key     Go type           Transformation
  ──────────────────────────────────────────────────────────────────────────
  destination  RouteDestination  → backendRefs[] shared by all rules in this RouteV3
  rules        []Rule            → one HTTPRoute rule per Rule entry

---

### RouteDestination

  JSON key       Go type         Transformation
  ──────────────────────────────────────────────────────────────────────────────────
  endpoint       string          → parse host + port for HTTPRoute.spec.rules[].backendRefs[].name and .port
  cluster        string          OMIT
  tlsSupported   bool            OMIT
  tlsEndpoint    string          OMIT
  httpVersion    *int32          OMIT
  tlsConfigName  string          OMIT
  circuitBreaker CircuitBreaker  OMIT
  tcpKeepalive   *TcpKeepalive   OMIT

#### Endpoint to backendRef resolution

Endpoint parsing — pattern: http://<name>:<port>
    
    name: everything between "http://" and last ":"
          preserve Helm expressions exactly
          e.g. http://{{ .Values.DEPLOYMENT_RESOURCE_NAME }}:8080
               → name: "{{ .Values.DEPLOYMENT_RESOURCE_NAME }}"
          e.g. http://public-gateway-service:8080
               → name: "public-gateway-service"
    port: number after last ":"
          e.g. :8080 → 8080

Examples:

    "http://{{ .Values.DEPLOYMENT_RESOURCE_NAME }}:8080" → host="{{ .Values.DEPLOYMENT_RESOURCE_NAME }}", port=8080
    "http://public-gateway-service:8080"                 → host="public-gateway-service",                 port=8080
    "my-service:9090"                                    → prepend http:// → host="my-service",            port=9090

Output:

```yaml
  backendRefs:
  - kind: Service
    name: <hostname from destination.endpoint>
    port: <port from destination.endpoint>
```

---

### Rule

  JSON key        Go type            Transformation
  ────────────────────────────────────────────────────────────────────────────────────────
  match           RouteMatch         → matches[] (see RouteMatch below)
  prefixRewrite   string             → URLRewrite filter path.ReplacePrefixMatch (when non-empty)
  hostRewrite     string             → URLRewrite filter hostname (when non-empty)
  addHeaders      []HeaderDefinition → RequestHeaderModifier add[] (rule-level, merged with VS-level)
  removeHeaders   []string           → RequestHeaderModifier remove[] (rule-level, merged with VS-level)
  timeout         *int64             → timeouts.request: "<value>ms"  (value is milliseconds)
  allowed         *bool              OMIT
  idleTimeout     *int64             OMIT  ⚠ flag for manual review if non-nil
  statefulSession *StatefulSession   OMIT  ⚠ flag for manual review if non-nil
  rateLimit       string             OMIT  ⚠ flag for manual review if non-empty
  deny            *bool              OMIT  ⚠ flag for manual review if non-nil
  luaFilter       string             OMIT  ⚠ flag for manual review if non-empty

---

### RouteMatch

  JSON key  Go type          Transformation
  ───────────────────────────────────────────────────────────────────────────────────────
  prefix    string           → path.type: PathPrefix,        value: <prefix>
  path      string           → path.type: Exact,             value: <path>
  regExp    string           → path.type: RegularExpression, value: <regexp>
  headers   []HeaderMatcher  → matches[].headers[]

  Path match type — mutually exclusive, apply first non-empty in this priority:
    1. prefix  → PathPrefix
    2. path    → Exact
    3. regExp  → RegularExpression

---

### HeaderMatcher

  JSON key  Go type  Transformation
  ──────────────────────────────────────────────────────────
  name      string   → matches[].headers[].name
  value     string   → matches[].headers[].value  (type: Exact)

---

### HeaderDefinition  (used in VirtualService.addHeaders and Rule.addHeaders)

  JSON key  Go type  Transformation
  ──────────────────────────────────────────────────────────────────────
  name      string   → RequestHeaderModifier.add[].name
  value     string   → RequestHeaderModifier.add[].value

---

### StatefulSession  (all fields — OMIT, flag for manual review)

  version, namespace, cluster, hostname, gateways, port,
  enabled, cookie, route, overridden
  → No Gateway API HTTPRoute equivalent.
  → Add comment: # ⚠ MANUAL REVIEW: statefulSession has no Gateway API equivalent

---

### Fields to OMIT in all Istio output

  RoutingConfigRequestV3:   namespace, listenerPort, tlsSupported, overridden
  VirtualService:           rateLimit (⚠), overridden
  RouteConfig:              version
  RouteDestination:         cluster, tlsSupported, tlsEndpoint, httpVersion,
                            tlsConfigName, circuitBreaker, tcpKeepalive
  Rule:                     allowed, idleTimeout (⚠), statefulSession (⚠),
                            rateLimit (⚠), deny (⚠), luaFilter (⚠)

### Fields to OMIT in all Istio output

  routeConfiguration.version
  rules[].allowed
  destination.cluster
  spec.env (on any CR)
  spec.allowVirtualHosts
  spec.gateway (on Gateway CR)
## Gateway

Every condition below is evaluated against a `Gateway` **CR present in the chart**. A gateway that
Step 2 resolves by well-known name alone — `egress-gateway`, `public-gateway-service`,
`private-gateway-service`, `internal-gateway-service` — has no CR here and produces **no** Istio
Gateway: the platform owns that object, and emitting a second one collides with it. Routes still
reference it through `parentRefs`, per
[parent-refs-resolution.md](parent-refs-resolution.md).

### § Gateway-to-Istio-Gateway

Condition:
  spec.gatewayType in [`ingress`, `egress`]

Input fields → Output fields:

  metadata.name             → metadata.name (copy exactly, preserve Helm expressions)
  metadata.labels           → metadata.labels (refer to common label resolution rules)
  spec.ingresses            → transform to k8s Ingress objects
  spec.port                 → spec.listeners[0].port
  spec.gatewayType          → OMIT (used for classification only)
  spec.gateway              → OMIT
  spec.allowVirtualHosts    → OMIT
  spec.env                  → OMIT
  spec.hpa                  → transform to k8s HPA objects
  spec.replicas             → OMIT
  spec.gatewayPorts         → spec.listeners[]
  spec.masterConfiguration  → OMIT

Notes:
  - port and gatewayPorts are mutually exclusive

Output template:
  apiVersion: gateway.networking.k8s.io/v1
  kind: Gateway
  metadata:
    name: '<metadata.name>'
    labels:
      <resolved labels>
  spec:
    gatewayClassName: istio
    listeners:
    - name: <'default' or gatewayPort[].name>
      port: <port or gatewayPort[].port>
      protocol: <'HTTP' or gatewayPort[].protocol in UPPER CASE>
      allowedRoutes:
        namespaces:
          from: Same

Multiple gatewayPort entries → multiple listeners:
  listeners:
  - name: <gatewayPort[0].name>
    port: <gatewayPort[0].port>
    protocol: <gatewayPort[0].protocol>
    allowedRoutes:
      namespaces:
        from: Same
  - name: <gatewayPort[1].name>
    port: <gatewayPort[1].port>
    protocol: <gatewayPort[1].protocol>
    allowedRoutes:
      namespaces:
        from: Same

### § Ingress for ingress gateway

For `ingress` gateway create Ingress 

Output Template:
```yaml
kind: Ingress
apiVersion: networking.k8s.io/v1
metadata:
  name: <gateway-name>-web
  labels:
    app.kubernetes.io/part-of: <take the same value as in gateway>
  ownerReferences:
    - apiVersion: gateway.networking.k8s.io/v1
      kind: Gateway
      name: <gateway-name>
spec:
  rules:
    - host: {{ printf "%s-%s.svc.cluster.local" "<gateway-name>" .Release.Namespace }}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: <gateway-name>-istio
                port:
                  number: 8080
```

### § Egress-Gateway-to-Istio-Gateway

Condition:
  spec.gatewayType is absent && `metadata.name` = `egress-gateway`

Transformation is basically like [§ Gateway-to-Istio-Gateway](#gateway-to-istio-gateway)
With one change:

  - Wrap source gateway in Core guard (same as all other Gateway CRs — see Step 3)

Keep the Istio Gateway listener HTTP. Core Mesh `TlsDef` on egress is **outbound**
TLS origination to the external host, not listener TLS — see
[tls-def-mapping.md](tls-def-mapping.md).

### § Gateway-to-null

Condition:
  spec.gatewayType in [`mesh`, absent]

Gateway is omitted. Routes will be transferred to waypoint

### Detect mesh Gateway name

IF spec.gatewayType in [`mesh`, absent]
  memorize `metadata.name` value as mesh Gateway name

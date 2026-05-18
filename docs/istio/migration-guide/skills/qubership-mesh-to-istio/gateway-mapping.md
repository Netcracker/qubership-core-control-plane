## Gateway

### § Gateway-to-Istio-Gateway

Condition:
  spec.gatewayType in [`ingress`, `egress`] OR 

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

### § Egress-Gateway-to-Istio-Gateway

Condition:
  spec.gatewayType is absent && `metadata.name` = `egress-gateway`

Transformation is basically like [§ Gateway-to-Istio-Gateway](#gateway-to-istio-gateway)
With several changes:

  - Add Service for egress-gateway to the same file where gateway placed

```yaml
{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}
apiVersion: v1
kind: Service
metadata:
  name: egress-gateway
spec:
  type: ClusterIP
  selector:
    gateway.networking.k8s.io/gateway-name: egress-gateway
  ports:
    <ports from listeners>
{{- end }}
```

### § Gateway-to-null

Condition:
  spec.gatewayType in [`mesh`, absent]

Gateway is omitted. Routes will be transferred to waypoint

### Detect mesh Gateway name

IF spec.gatewayType in [`mesh`, absent]
  memorize `metadata.name` value as mesh Gateway name

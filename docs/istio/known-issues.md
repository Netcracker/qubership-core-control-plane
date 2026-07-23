# Istio Ambient Mesh Known Issues

## Default Header Host Modification Behavior

In most cases, Cloud Core gateways in Istio Ambient Mesh keep the original behavior of Cloud Core Service Mesh regarding default HTTP header modification in gateways.

However, there is one scenario where the behavior differs: the `Host` header is set by Istio gateways differently on public, private, and internal gateways.

In Cloud Core Service Mesh, public, private, and internal gateways utilized the `host_auto_rewrite: true` setting of envoyproxy, which automatically rewrites the `Host` header to the target service `<hostname>:<port>`.

In Istio, it is impossible to utilize the same envoyproxy setting, because Istio uses envoyproxy EDS (Endpoint Discovery Service) and istiod does not populate the EDS hostname field for k8s services. Instead, the default envoyproxy behavior for the `Host` header is preserved:
  * At an ingress gateway (public or private gateway), incoming requests have an external Host header (e.g., api.example.com). When the gateway routes to a backend ClusterIP service, the Host stays as api.example.com unless you
  explicitly rewrite it;
  * At an internal gateway or waypoint, the original Host requested by the client is preserved, e.g. `internal-gateway-service:8080` or `my-backend-srv:8080`.

## Double Slashes in Path

Legacy Cloud-Core Service Mesh merges duplicated slashes in a path into a single slash, e.g. `dbaas:8080//api/v1/sadsd` -> `dbaas:8080/api/v1/sadsd`.

Istio, like most systems, considers a double slash to be a different path, so it does not merge slashes into one, which in some cases can lead to 404 errors. Such issues should be fixed on the client side by correcting the request path.

## HTTPRoutes for Headless Services

For headless services (services with `clusterIP: None`), the HTTPRoute must have an explicitly set hostname matcher, otherwise the route will not work.

Example of the correct HTTPRoute configuration for headless service:
```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: '{{ .Values.SERVICE_NAME }}-routes'
  labels:
    app.kubernetes.io/name: '{{ printf "%s-routes" .Values.SERVICE_NAME }}'
    app.kubernetes.io/instance: '{{ printf "%s-routes-%s" .Values.SERVICE_NAME .Values.NAMESPACE | trunc 63 | trimSuffix "-" }}'
    app.kubernetes.io/version: "{{ .Chart.AppVersion }}"
    app.kubernetes.io/component: '{{ .Values.SERVICE_NAME }}'
    app.kubernetes.io/part-of: 'Cloud-Core'
    app.kubernetes.io/managed-by: 'Helm'
    app.kubernetes.io/processed-by-operator: 'istiod'
spec:
  parentRefs:
    - name: {{ .Values.SERVICE_NAME}}
      kind: Service
      group: ""
      port: 8080
  hostnames:
    - '{{ .Values.SERVICE_NAME }}'
    - '{{ .Values.SERVICE_NAME }}.{{ .Values.NAMESPACE }}'
    - '{{ .Values.SERVICE_NAME }}.{{ .Values.NAMESPACE }}.svc'
    - '{{ .Values.SERVICE_NAME }}.{{ .Values.NAMESPACE }}.svc.cluster.local'
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: "/api/v1/control-plane/routes"
      filters:
        - type: URLRewrite
          urlRewrite:
            path:
              type: ReplacePrefixMatch
              replacePrefixMatch: "/api/v1/routes/
      backendRefs:
        - name: {{ .Values.SERVICE_NAME }}
          port: 8080
```
# Forbid Routes on Public or Private Gateway

This article describes capability to forbid some endpoints by prefix on Public or Private gateway. 

This is done by `deny` field in `RouteConfiguration` - see the example below. 

You need to create CR in k8s with kind `Mesh` and subKind `RouteConfiguration`: 

```yaml
---
apiVersion: core.netcracker.com/v1
kind: Mesh
subKind: RouteConfiguration
metadata:
  name: {{ .Values.SERVICE_NAME }}-forbidden-public-routes
  namespace: {{ .Values.NAMESPACE }}
  labels:
    deployer.cleanup/allow: "true"
    app.kubernetes.io/managed-by: saasDeployer
    deployment.netcracker.com/sessionId: {{ .Values.DEPLOYMENT_SESSION_ID }}
    app.kubernetes.io/part-of: My-Application-Name
    app.kubernetes.io/processed-by-operator: "core-operator"
spec:
  gateways: ["public-gateway-service"]
  virtualServices:
  - name: public-gateway-service
    hosts: ["*"]
    routeConfiguration:
      routes:
      - destination: # destination can be any existing cluster and its endpoint, it is needed to comply with RouteConfiguration structure
          cluster: "{{ .Values.SERVICE_NAME }}"
          endpoint: http://{{ .Values.DEPLOYMENT_RESOURCE_NAME }}:8080
        rules:
        - match:
            prefix: /swagger
          deny: true # this flag forbids all endpoints on gateway starting with the provided prefix (/swagger)
        - match:
            prefix: /api/v1/my-allowed-route # allowed routes also can be registered in the same configuration
```

In this example all endpoints starting with prefix `/swagger` will return 404, e.g. `/swagger`, `/swagger/ui`, etc. 

Route with `deny: true` will have higher priority than any other route starting with the same prefix. 

You need to explicitly delete route with `deny: true` to make such endpoints accessible on gateway again. See [routes deletion guide](./routes-deletion-guide.md).


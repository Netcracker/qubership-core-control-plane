## § FacadeService

### FacadeService-to-Service

Input fields → Output fields:

  metadata.name             → metadata.name (copy exactly, preserve Helm expressions)
  metadata.labels           → metadata.labels (copy ALL; omit block if empty)
  metadata.annotations      → metadata.annotations (copy ALL; omit block if empty)
  spec.port + spec.gatewayPorts → spec.ports[] (see port mapping priority below)
  spec.env                  → OMIT
  spec.gateway              → OMIT
  spec.gatewayType          → OMIT
  spec.allowVirtualHosts    → OMIT
  spec.replicas             → OMIT
  spec.masterConfiguration  → OMIT
  spec.hpa                  → OMIT
  spec.ingresses            → OMIT

Port mapping priority:
  IF spec.gatewayPorts[] is non-empty:
    generate one port entry per GatewayPorts item:
      name:       GatewayPorts.name  (use "web" if name is empty)
      port:       GatewayPorts.port
      targetPort: GatewayPorts.port
      protocol:   GatewayPorts.protocol in UPPER CASE (omit field if empty)
  ELSE IF spec.port > 0:
    generate single port entry:
      name: web
      port: spec.port
      targetPort: spec.port
  ELSE:
    generate no ports + add: # ⚠ MANUAL REVIEW: no port defined

NO spec.selector field — Service is used as HTTPRoute parent only.

Output template (spec.port only, no gatewayPorts):
  kind: Service
  apiVersion: v1
  metadata:
    name: '<metadata.name>'
    labels:                         <- omit block if source has no labels
      <copy all verbatim>
    annotations:                    <- omit block if source has no annotations
      <copy all verbatim>
  spec:
    ports:
    - name: web
      port: <spec.port>
      targetPort: <spec.port>
    selector:
      name: '{{ .Values.DEPLOYMENT_RESOURCE_NAME }}'

Output template (gatewayPorts present):
  kind: Service
  apiVersion: v1
  metadata:
    name: '<metadata.name>'
    labels:
      <copy all verbatim>
    annotations:
      <copy all verbatim>
  spec:
    ports:
    - name: <gatewayPorts[0].name>
      port: <gatewayPorts[0].port>
      targetPort: <gatewayPorts[0].port>
      protocol: <gatewayPorts[0].protocol>   <- omit line if protocol is empty
    - name: <gatewayPorts[1].name>
      port: <gatewayPorts[1].port>
      protocol: <gatewayPorts[1].protocol>   <- omit line if protocol is empty
    selector:
      name: '{{ .Values.DEPLOYMENT_RESOURCE_NAME }}'      

### Detect mesh Gateway name
IF FacadeService contains `spec.gateway` field
  memorize `spec.gateway` value as corresponding mesh Gateway name
ELSE 
  memorize `metadata.name` + "-gateway" as corresponding mesh Gateway name
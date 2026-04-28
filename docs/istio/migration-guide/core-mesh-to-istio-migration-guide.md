# Service Mesh Migration Guide
**Cloud-Core Mesh → Istio**

---

## Overview

This guide walks you through migrating your service from the Cloud-Core mesh to Istio. Follow the steps in order. Each step builds on the previous one.

### Skills

This guide relies on AI skills that automate the bulk of the migration:

| Skill | Purpose |
|---|---|
| [`core-mesh-to-istio-migration`](skills/core-mesh-to-istio-migration/SKILL.md) | **Orchestrator** — runs the full migration end-to-end, delegates to the two skills below, and maintains a `MIGRATION_LOG.md` of Done / Skipped / Needs review items |
| [`qubership-mesh-to-istio`](skills/qubership-mesh-to-istio/SKILL.md) | Convert existing Helm mesh CRs (FacadeService, Gateway, RouteConfiguration) to Istio Gateway API resources |
| [`httproute-from-code`](skills/httproute-from-code/SKILL.md) | Generate HTTPRoute CRs from Go/Java route registration source code |

**Recommended:** run the orchestrator skill (`core-mesh-to-istio-migration`) and let it drive the steps below. The two atomic skills remain available if you prefer to run a single step in isolation.

---

## Step 1 — Migrate declarative routes - existing Mesh CRs to HTTPRoute CRs

Use the skill [`qubership-mesh-to-istio`](skills/qubership-mesh-to-istio/SKILL.md) to automatically convert your existing mesh custom resources (`FacadeService`, `Gateway`, `RouteConfiguration`) into GatewayAPI HTTPRoute manifests while keeping the chart deployable on both mesh types.

### How to run

1. Open your IDE in your repository root.
2. Ask AI to migrate your Helm chart to Istio, e.g.: *"Migrate mesh CRs to Istio in `<path-to-your-helm-chart>`"*
3. The skill will:
    - Wrap all original `FacadeService`, `Gateway`, and `RouteConfiguration` CRs in `{{- if eq .Values.SERVICE_MESH_TYPE "Core" }}` guards
    - Generate new `-istio.yaml` files alongside each original (e.g. `gateway.yaml` → `gateway-istio.yaml`) wrapped in `{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}`
    - Convert `Gateway` (ingress/egress type) → Istio `Gateway` + `HTTPRoute`
    - Convert `RouteConfiguration` → `HTTPRoute` with correct `parentRefs` (Gateway or Service depending on gateway type)
    - Omit `FacadeService` and `Gateway` (mesh type) from Istio output — their routes become east-west `HTTPRoute` resources with Service `parentRefs`
    - Add `SERVICE_MESH_TYPE: Core` to `values.yaml` and update `values.schema.json`
4. Review the generated `-istio.yaml` files and the modified originals.


Render helm chart and review the result

```bash
helm template . --set SERVICE_MESH_TYPE=Istio | grep -E 'kind: (HTTPRoute|Gateway)'
```

---

## Step 1.1 — Manually Handle Flagged Features

After the skill runs, it prints a summary of items it could not migrate automatically, marked with `# ⚠ MANUAL REVIEW REQUIRED`. Find the way to migrate them manually.
                                         
---

## Step 2 — Migrate non-declarative routes (registered with routes-registration libraries)

---

## Step 2.1 — Switch to the New Mesh-Type-Aware Routes-Registration Libraries

Replace the old route-posting libraries versions with the new mesh-aware versions. The new libraries versions read the `SERVICE_MESH_TYPE` env var at runtime and skip posting routes to the Cloud-Core Control-Plane when Istio mesh is specified. Istio handles routing directly from the HTTPRoute CRs instead. 

### What changes at runtime

| Condition                 | Old version library           | New version library                         |
|---------------------------|-------------------------------|---------------------------------------------|
| `SERVICE_MESH_TYPE=Istio` | Posts routes to control plane | Skips control plane — Istio handles routing |
| `SERVICE_MESH_TYPE=Core` (or any value other than Istio) | Posts routes to control plane | Posts routes to control plane (unchanged)   |


### Java — update your dependency

#### Spring

```xml
<dependency>
    <groupId>com.netcracker.cloud</groupId>
    <artifactId>route-registration-webclient</artifactId>
    <version>7.1.0<!-- should be >=7.1.0 --></version>
</dependency>

OR 

<dependency>
    <groupId>com.netcracker.cloud</groupId>
    <artifactId>route-registration-resttemplate</artifactId>
    <version>7.1.0<!-- should be >=7.1.0 --></version>
</dependency>
```

Using rest libraries BOM

```xml
 <dependencyManagement>
        <dependencies>
            <dependency>
                <groupId>com.netcracker.cloud</groupId>
                <artifactId>rest-libraries-bom</artifactId>
                <version>7.1.0<!-- should be >=7.1.0 --></version>
                <scope>import</scope>
                <type>pom</type>
            </dependency>
        </dependencies>
</dependencyManagement>
```

Using common BOM

```xml
    <dependencyManagement>
        <dependencies>
            <dependency>
                <groupId>com.netcracker.cloud</groupId>
                <artifactId>cloud-core-java-bom</artifactId>
                <version>12.0.2<!-- should be >=12.0.2 --></version>
                <type>pom</type>
                <scope>import</scope>
            </dependency>
        </dependencies>
    </dependencyManagement>
```

#### Quarkus

```xml
<dependency>
    <groupId>com.netcracker.cloud.quarkus</groupId>
    <artifactId>routes-registrator</artifactId>
    <version>9.1.0<!-- should be >=9.1.0 --></version>
</dependency>
```

Using BOM

```xml
    <dependencyManagement>
        <dependencies>
            <dependency>
                <groupId>com.netcracker.cloud</groupId>
                <artifactId>cloud-core-quarkus-bom-publish</artifactId>
                <version>9.1.0<!-- should be >=9.1.0 --></version>
                <type>pom</type>
                <scope>import</scope>
            </dependency>
        </dependencies>
    </dependencyManagement>


```


### Go — update your go.mod

```go
require (
    ...
    github.com/netcracker/qubership-core-lib-go-rest-utils/v2 v2.5.0 // should be  >= 2.5.0
    ...
)
```

```go
import (
    ...
	routeregistration "github.com/netcracker/qubership-core-lib-go-rest-utils/v2/route-registration"
    ...
)
```

## Step 2.2 — Set the SERVICE_MESH_TYPE Environment Variable

All services (Java and Go) using route registration libraries should have the environment variable that controls which mesh implementation is active. 
By default set it to enable `Core` mode for compatibility with environments, where Istio is not yet installed:

```
SERVICE_MESH_TYPE=Core
```

### Where to set it


**Kubernetes Deployment manifest**:

```yaml
kind: Deployment
apiVersion: apps/v1
spec:
    template:
        spec:
            containers:
                  env:
                      - name: SERVICE_MESH_TYPE
                        value: '{{ .Values.SERVICE_MESH_TYPE }}'                        
```

**values.yaml**:
```yaml
SERVICE_MESH_TYPE: "Core"                  
```

**values.schema.json**:
```yaml
{ 
  ...
  "examples": [
    {
      "SERVICE_MESH_TYPE": "Core"
    }
  ],
  "properties": {
    ...
    "SERVICE_MESH_TYPE": {
      "$id": "#/properties/SERVICE_MESH_TYPE",
      "type": "string",
      "title": "The SERVICE_MESH_TYPE schema",
      "description": "Service mesh type. Use Core for Qubership Cloud Core Mesh or Istio for Istio Ambient Mesh.",
      "enum": ["Core", "Istio"],
      "default": "Core",
      "examples": [
        "Core",
        "Istio"
      ]
    }
  },
  "additionalProperties": true
}
```

## Step 2.3 — Add the Maven Plugin (Java services only)

If your Java service uses route-registration annotations:
1. Add the route-registration [Maven plugin](https://github.com/Netcracker/qubership-core-control-plane/blob/main/httproute-generator/README.md). 
This plugin scans compiled classes for routing annotations - and generates equivalent HTTPRoutes to provide consistent routing in Istio mesh. 
2. Specify `outputFile` path in helm chart templates folder. 
3. Build the project. 
4. Commit `outputFile` to your branch. 
5. In case of any changes in routes annotations - do not forget to build locally and commit changes of routes produced by plugin.

### Add to pom.xml

```xml
<build>
    <plugins>
        <plugin>
            <groupId>org.qubership.cloud.core</groupId>
            <artifactId>httproutes-generator-maven-plugin</artifactId>
            <version>0.0.1-SNAPSHOT</version>
            <executions>
                <execution>
                    <goals>
                        <goal>generate-routes</goal>
                    </goals>
                </execution>
            </executions>
            <configuration>
                <packages>
                    <package>com.example</package>
                </packages>
                <servicePort>8080</servicePort>
                <outputFile>helm-templates/service/templates/annotations-httproutes.yaml</outputFile>
                <backendRefVal>{{ .Values.DEPLOYMENT_RESOURCE_NAME }}</backendRefVal>
            </configuration>
        </plugin>
    </plugins>
</build>
```

After adding the plugin, run a local build to confirm it passes:

```bash
mvn clean compile
```

---

## Step 2.4 — Generate HTTPRoute CRs from Route Registration Code

Use the skill [`httproute-from-code`](skills/httproute-from-code/SKILL.md) to produce HTTPRoute manifests directly from your application source code. This ensures your code-defined routes are reflected in the cluster configuration.

### How to run

1. Open your IDE.
2. Run the slash command: `/httproute-from-code <path>` pointing at your directory or file with route registration code.
3. The skill scans Go and Java files, detects route definitions, and outputs HTTPRoute CRs.
4. Generated files are saved to `helm-templates/<service name>/templates/source-code-httproutes.yaml`.
5. Review the summary table printed by the skill. Resolve any errors before continuing.
6. Commit all generated files to your branch.
7. In case of any changes in routes registration code - do not forget to run `httproute-from-code` skill and commit changes of routes produced by skill.

> **Tip:** You can run the skill against a single file or an entire directory.

---

## Step 2.5 — Verify All Routes Are Wrapped in Istio Conditionals

Every HTTPRoute CR in your Helm templates must be wrapped in a conditional guard so it only renders when Istio is the active mesh. This prevents Gateway API resources from being applied to clusters running Core mesh.

### Required pattern

```yaml
{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-service-public-routes
  namespace: {{ .Values.NAMESPACE }}
spec:
  ...
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-service-private-routes
  ...
{{- end }}
```

### Validation

To verify, do a Helm dry-run with Istio disabled and confirm no HTTPRoute objects are rendered:

```bash
helm template . --set SERVICE_MESH_TYPE=Core | grep 'kind: HTTPRoute'
# Expected: no output
```

---

## Final Checklist

Before raising a PR, verify all of the following:

- [ ] [`qubership-mesh-to-istio`](skills/qubership-mesh-to-istio/SKILL.md) skill run: existing mesh CRs converted to HTTPRoute CRs
- [ ] Flagged features from Step 1.1 manually resolved
- [ ] New mesh-aware libraries replace old route-posting libraries
- [ ] `SERVICE_MESH_TYPE=Istio` set in Helm values / Deployment
- [ ] Maven plugin added and local build passes (Java only)
- [ ] [`httproute-from-code`](skills/httproute-from-code/SKILL.md) skill run: route registration code converted to HTTPRoute CRs
- [ ] All HTTPRoute CRs wrapped in a `{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}` block

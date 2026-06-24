# Service Mesh Migration Guide
**Cloud-Core Mesh → Istio**

---

## Overview

This guide covers migrating your service from the Cloud-Core mesh to Istio.

There are **two ways** to perform the migration. Pick one:

### Option A — Run the orchestrator skill (recommended)

Let the [`core-mesh-to-istio-migration`](../../../agent-packages/core-mesh-to-istio-migration/.apm/skills/core-mesh-to-istio-migration/SKILL.md)
orchestrator skill drive the entire migration end-to-end. It runs every step
below in order, delegates to the two atomic skills, validates the result, and
maintains a `MIGRATION_LOG.md` of Done / Skipped / Needs review items so nothing
is missed.

1. Install the skills (see [Install the skills](#install-the-skills-apm)) and open
   your IDE in your repository root.
2. Ask the agent to run the migration by name, e.g.: *"Run the
   core-mesh-to-istio-migration skill on `<path-to-your-helm-chart>`"*.
3. Answer any questions it asks (chart path, source-code path, language), then
   review `MIGRATION_LOG.md` — especially every **Needs review** entry — before
   raising a PR.
4. Reuse values captured in `MIGRATION_LOG.md` from Step 1:
   - detected `backendRefName` / `backendRefPort`,
   - detected output labels for generated Gateway/HTTPRoute resources.
   The orchestrator propagates these into Maven plugin configuration and
   `httproute-from-code` so all generated routes stay consistent.

This is the preferred path: it is faster, applies the steps consistently, and
produces an auditable log.

### Option B — Follow this guide manually

Work through the steps below yourself, in order (each builds on the previous).
Along the way you can still invoke the two atomic skills to automate individual
steps, or do the edits by hand. Choose this path when you want full control,
need to run a single step in isolation, or are migrating something the
orchestrator cannot fully handle.

### Skills used by both options

| Skill | Purpose |
|---|---|
| [`core-mesh-to-istio-migration`](../../../agent-packages/core-mesh-to-istio-migration/.apm/skills/core-mesh-to-istio-migration/SKILL.md) | **Orchestrator** (Option A) — runs the full migration end-to-end, delegates to the two skills below, and maintains a `MIGRATION_LOG.md` of Done / Skipped / Needs review items |
| [`core-mesh-crs-to-gatewayapi`](../../../agent-packages/core-mesh-crs-to-gatewayapi/.apm/skills/core-mesh-crs-to-gatewayapi/SKILL.md) | Convert existing Helm mesh CRs (FacadeService, Gateway, RouteConfiguration) to Istio Gateway API resources |
| [`httproute-from-code`](../../../agent-packages/httproute-from-code/.apm/skills/httproute-from-code/SKILL.md) | Generate HTTPRoute CRs from Go/Java route registration source code |

> The rest of this guide documents the individual steps. With **Option A** the
> orchestrator performs them for you; with **Option B** you follow them
> manually. Either way, finish with the [Final Checklist](#final-checklist).

### Install the skills (APM)

The skills are distributed as [APM](https://github.com/microsoft/apm) packages
under [`agent-packages/`](../../../agent-packages). Install the orchestrator into
your repo — it pulls in the two atomic skills (and the shared
`path-specificity-sorting` procedure) as dependencies:

```sh
apm install Netcracker/qubership-core-control-plane/agent-packages/core-mesh-to-istio-migration --target claude
```

Swap `--target claude` for your agent (`cursor`, `copilot`, `codex`, …). This
deploys the skills under `.claude/skills/` (and the trigger rules under
`.claude/rules/`), after which the agent can run them by name. Re-run the command
to pick up a new version. 

---

## Step 1 — Migrate declarative routes - existing Mesh CRs to HTTPRoute CRs

Use the skill [`core-mesh-crs-to-gatewayapi`](../../../agent-packages/core-mesh-crs-to-gatewayapi/.apm/skills/core-mesh-crs-to-gatewayapi/SKILL.md) to automatically convert your existing mesh custom resources (`FacadeService`, `Gateway`, `RouteConfiguration`) into GatewayAPI HTTPRoute manifests while keeping the chart deployable on both mesh types.

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
    - Detect and report migration-wide backend reference (`backendRefName` / `backendRefPort`)
    - Infer and report output labels applied to generated Gateways/HTTPRoutes
4. Review the generated `-istio.yaml` files and the modified originals.


Render helm chart and review the result

```bash
helm template . --set SERVICE_MESH_TYPE=Istio | grep -E 'kind: (HTTPRoute|Gateway)'
```

---

## Step 1.1 — Manually Handle Flagged Features

After the skill runs, it prints a summary of items it could not migrate automatically, marked with `# ⚠ MANUAL REVIEW REQUIRED`. Find the way to migrate them manually.

Also review `MIGRATION_LOG.md` entries for:
- unresolved backend reference,
- unresolved output labels.
                                         
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
      "description": "Service mesh type. Use `Core` for Cloud Core Mesh or `Istio` for Istio Ambient Mesh.",
      "enum": ["Istio", "Core"],
      "default": "Core",
      "internal": true
    }
  },
  "additionalProperties": true
}
```

## Step 2.3 — Add the Maven Plugin (Java services only)

If your Java service uses route-registration annotations:
1. Add the route-registration [Maven plugin](https://github.com/Netcracker/qubership-core-java-libs/blob/main/core-maven-plugins/httproutes-generator-maven-plugin/README.md). 
This plugin scans compiled classes for routing annotations - and generates equivalent HTTPRoutes to provide consistent routing in Istio mesh. 
2. Specify `outputFile` path in helm chart templates folder. 
3. Build the project. 
4. Commit `outputFile` to your branch. 
5. In case of any changes in routes annotations - do not forget to build locally and commit changes of routes produced by plugin.

Before adding plugin configuration, resolve inputs from Step 1 (`MIGRATION_LOG.md`):

- `backendRefName` / `backendRefPort` (preferred: detected values),
- `routeLabels` map (preferred: detected output labels).

If Step 1 left any of these unresolved, provide them explicitly before
continuing.

### Add to pom.xml

Use plugin coordinates `com.netcracker.cloud.plugins:httproutes-generator-maven-plugin`
and pick the latest available plugin version, but never lower than `1.0.2`.

```xml
<build>
    <plugins>
        <plugin>
            <groupId>com.netcracker.cloud.plugins</groupId>
            <artifactId>httproutes-generator-maven-plugin</artifactId>
            <version><!-- use latest available, but >= 1.0.2 --></version>
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
                <servicePort><!-- use detected backendRefPort from Step 1, fallback 8080 --></servicePort>
                <outputFile>helm-templates/service/templates/annotations-httproutes.yaml</outputFile>
                <backendRefVal><!-- use detected backendRefName from Step 1 --></backendRefVal>
                <labels>
                    <!-- use detected output labels from Step 1 -->
                    <label>
                        <key>app.kubernetes.io/name</key>
                        <value>{{ .Values.SERVICE_NAME }}</value>
                    </label>
                    <label>
                        <key>app.kubernetes.io/part-of</key>
                        <value>{{ .Values.APPLICATION_NAME }}</value>
                    </label>
                    <label>
                        <key>app.kubernetes.io/managed-by</key>
                        <value>{{ .Values.MANAGED_BY }}</value>
                    </label>
                    <label>
                        <key>deployment.netcracker.com/sessionId</key>
                        <value>{{ .Values.DEPLOYMENT_SESSION_ID }}</value>
                    </label>
                    <label>
                        <key>deployer.cleanup/allow</key>
                        <value>true</value>
                    </label>
                    <label>
                        <key>app.kubernetes.io/processed-by-operator</key>
                        <value>istiod</value>
                    </label>
                </labels>
            </configuration>
        </plugin>
    </plugins>
</build>
```

After adding the plugin, run a local build to confirm it passes:

```bash
mvn clean process-classes
```

---

## Step 2.4 — Generate HTTPRoute CRs from Route Registration Code

Use the skill [`httproute-from-code`](../../../agent-packages/httproute-from-code/.apm/skills/httproute-from-code/SKILL.md) to produce HTTPRoute manifests directly from your application source code. This ensures your code-defined routes are reflected in the cluster configuration.

### How to run

1. Open your IDE.
2. Ask the agent to run the `httproute-from-code` skill on your directory or file
   with route registration code, and pass:
   - `backendRefName`,
   - `backendRefPort`,
   - `routeLabels` (detected output labels from Step 1).
3. The skill scans Go and Java files, detects route definitions, and outputs HTTPRoute CRs.
4. Generated files are saved to `helm-templates/<service name>/templates/source-code-httproutes.yaml`.
5. Review the summary table printed by the skill. Resolve any errors before continuing.
6. Commit all generated files to your branch.
7. In case of any changes in routes registration code - do not forget to run `httproute-from-code` skill and commit changes of routes produced by skill.

> **Tip:** You can run the skill against a single file or an entire directory.

If `routeLabels` is not passed, the skill applies the default label set:

- `app.kubernetes.io/name: {{ .Values.SERVICE_NAME }}`
- `app.kubernetes.io/part-of: {{ .Values.APPLICATION_NAME }}`
- `app.kubernetes.io/managed-by: {{ .Values.MANAGED_BY }}`
- `deployment.netcracker.com/sessionId: {{ .Values.DEPLOYMENT_SESSION_ID }}`
- `deployer.cleanup/allow: "true"`
- `app.kubernetes.io/processed-by-operator: istiod`

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

## Step 2.6 — Detect Duplicate HTTPRoute Rules

Once all HTTPRoutes exist — both those converted from declarative mesh CRs
(Step 1) and those generated from route registration code (Step 2.4 / the Maven
plugin) — the same route can end up defined twice. A common case: a route is
declared in a `RouteConfiguration` CR **and** also registered in application
code, so both pipelines emit a rule for it.

Scan all Istio-guarded HTTPRoutes and flag any **duplicate rules** — two or more
rules that share **at least one common parent** (`parentRefs[]` entry:
`group` + `kind` + `name`) **and** have **equal match parameters** (path match
`type` + `value`, plus any `headers` / `queryParams` / `method`). Compare
`{{ .Values.* }}` expressions as literal text.

These duplicates are **not removed automatically** — deleting the wrong copy can
silently change routing behaviour. For each duplicate group, record a manual
review item listing:

- the shared parent(s) and the match value,
- every file + HTTPRoute name (and rule) that contributes a copy,
- which source should remain authoritative (the `RouteConfiguration` CR or the
  registration code) so you can remove the redundant rule before merging.

Routes that share the same match but target only **different** parents are not
duplicates and need no action.

---

## Final Checklist

Before raising a PR, verify all of the following:

- [ ] [`core-mesh-crs-to-gatewayapi`](../../../agent-packages/core-mesh-crs-to-gatewayapi/.apm/skills/core-mesh-crs-to-gatewayapi/SKILL.md) skill run: existing mesh CRs converted to HTTPRoute CRs
- [ ] Flagged features from Step 1.1 manually resolved
- [ ] New mesh-aware libraries replace old route-posting libraries
- [ ] `SERVICE_MESH_TYPE=Istio` set in Helm values / Deployment
- [ ] Maven plugin added and local build passes (Java only)
- [ ] [`httproute-from-code`](../../../agent-packages/httproute-from-code/.apm/skills/httproute-from-code/SKILL.md) skill run: route registration code converted to HTTPRoute CRs
- [ ] All HTTPRoute CRs wrapped in a `{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}` block
- [ ] HTTPRoutes scanned for duplicate rules (same parent + equal match); duplicates resolved or flagged for review

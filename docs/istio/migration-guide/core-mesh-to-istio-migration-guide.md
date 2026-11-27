# Service Mesh Migration Guide
**Cloud-Core Mesh → Istio**

---

## Overview

This guide describes migration from legacy Cloud Core Service Mesh to Istio Ambient Mesh.

## Prerequisites

### Istio

Qubership Istio distribution should be installed on the cluster. See [qubership-istio installation notes](https://github.com/Netcracker/qubership-istio/blob/main/docs/public/installation.md).

### Environment Enrollment into Mesh

Having Istio installed on the cluster is not enough to enable Istio on the environment - you need to enroll namespace into mesh.
Follow this guide to enroll namespace into Istio Ambient Mesh: [namespace enrollment](https://github.com/Netcracker/qubership-istio/blob/main/docs/public/namespace-enrollment.md).

All namespaces of the same Composite Deployment must be enrolled into the mesh simultaneously.

### Cloud-Core

Cloud-Core supports Istio since version 10.4.x (release 27.1).

Migration of the existing environment to Istio must be performed via 2 sequentual deployment sessions:
1. Deploy Cloud-Core 9.27.2+ with parameter `SERVICE_MESH_TYPE=Istio`;
2. Deploy other migrated apps.

## Service Migration

This section covers migrating your service from the Cloud-Core mesh to Istio.

There are **two ways** to perform the migration. Pick one:

### Option A — Run the orchestrator skill (recommended)

Let the [`core-mesh-to-istio-migration`](../../../agent-packages/core-mesh-to-istio-migration/.apm/skills/core-mesh-to-istio-migration/SKILL.md)
orchestrator skill drive the entire migration end-to-end. It runs every step
below in order, delegates to the five atomic skills, validates the result, and
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
Along the way you can still invoke the atomic skills to automate individual
steps, or do the edits by hand. Choose this path when you want full control,
need to run a single step in isolation, or are migrating something the
orchestrator cannot fully handle.

### Skills used by both options

| Skill | Purpose |
|---|---|
| [`core-mesh-to-istio-migration`](../../../agent-packages/core-mesh-to-istio-migration/.apm/skills/core-mesh-to-istio-migration/SKILL.md) | **Orchestrator** (Option A) — runs the full migration end-to-end, delegates to the skills below, and maintains a `MIGRATION_LOG.md` of Done / Skipped / Needs review items |
| [`core-mesh-crs-to-istio`](../../../agent-packages/core-mesh-crs-to-istio/.apm/skills/core-mesh-crs-to-istio/SKILL.md) | Convert existing Helm mesh CRs (FacadeService, Gateway, RouteConfiguration) to Istio Gateway API resources |
| [`mesh-build-wiring`](../../../agent-packages/mesh-build-wiring/.apm/skills/mesh-build-wiring/SKILL.md) | Switch to mesh-aware route-registration libraries, wire `SERVICE_MESH_TYPE`, add the Maven plugin |
| [`httproute-from-code`](../../../agent-packages/httproute-from-code/.apm/skills/httproute-from-code/SKILL.md) | Generate HTTPRoute CRs from Go/Java route registration source code |
| [`istio-migration-validate`](../../../agent-packages/istio-migration-validate/.apm/skills/istio-migration-validate/SKILL.md) | Verify Istio guards, duplicate rules, access-control lint, imperative control-plane calls |
| [`public-gateway-authz-policy`](../../../agent-packages/public-gateway-authz-policy/.apm/skills/public-gateway-authz-policy/SKILL.md) | Recommend (and, only on request, draft) the public-gateway `AuthorizationPolicy` |

> The rest of this guide documents the individual steps. With **Option A** the
> orchestrator performs them for you; with **Option B** you follow them
> manually. Either way, finish with the [Final Checklist](#final-checklist).

### Install the skills (APM)

The skills are distributed as [APM](https://github.com/microsoft/apm) packages
under [`agent-packages/`](../../../agent-packages). Install the orchestrator into
your repo — it pulls in the five atomic skills above (and the shared
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

Use the skill [`core-mesh-crs-to-istio`](../../../agent-packages/core-mesh-crs-to-istio/.apm/skills/core-mesh-crs-to-istio/SKILL.md) to automatically convert your existing mesh custom resources (`FacadeService`, `Gateway`, `RouteConfiguration`) into GatewayAPI HTTPRoute manifests while keeping the chart deployable on both mesh types.

### How to run

1. Open your IDE in your repository root.
2. Ask AI to migrate your Helm chart to Istio, e.g.: *"Migrate mesh CRs to Istio in `<path-to-your-helm-chart>`"*
3. The skill will:
    - Wrap all original `FacadeService`, `Gateway`, and `RouteConfiguration` CRs in `{{- if eq .Values.SERVICE_MESH_TYPE "Core" }}` guards
    - Generate new `-istio.yaml` files alongside each original (e.g. `gateway.yaml` → `gateway-istio.yaml`) wrapped in `{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}`
    - Convert `Gateway` (ingress/egress type) → Istio `Gateway` + `HTTPRoute`
    - Convert `RouteConfiguration` → `HTTPRoute` with correct `parentRefs` (Gateway or Service depending on gateway type)
    - Convert `FacadeService` → Kubernetes `Service` and omit `Gateway` (mesh type) from Istio output — their routes become east-west `HTTPRoute` resources with Service `parentRefs`
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
    <version>7.6.2<!-- should be >=7.6.2 --></version>
</dependency>

OR 

<dependency>
    <groupId>com.netcracker.cloud</groupId>
    <artifactId>route-registration-resttemplate</artifactId>
    <version>7.6.2<!-- should be >=7.6.2 --></version>
</dependency>
```

Using rest libraries BOM

```xml
 <dependencyManagement>
        <dependencies>
            <dependency>
                <groupId>com.netcracker.cloud</groupId>
                <artifactId>rest-libraries-bom</artifactId>
                <version>7.6.2<!-- should be >=7.6.2 --></version>
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
                <version>12.3.2<!-- should be >=12.3.2 --></version>
                <type>pom</type>
                <scope>import</scope>
            </dependency>
        </dependencies>
    </dependencyManagement>
```

#### Quarkus

The effective `routes-registrator` version is the floor from
[`mesh-build-wiring`](../../../agent-packages/mesh-build-wiring/.apm/skills/mesh-build-wiring/SKILL.md):
`>= 10.4.2` with or without `com.netcracker.cloud.plugins:httproutes-generator-maven-plugin`.
Both recipes below have to resolve to `10.4.2`.

```xml
<dependency>
    <groupId>com.netcracker.cloud.quarkus</groupId>
    <artifactId>routes-registrator</artifactId>
    <version>10.4.2<!-- >= 10.4.2 --></version>
</dependency>
```

Using BOM

```xml
    <dependencyManagement>
        <dependencies>
            <dependency>
                <groupId>com.netcracker.cloud</groupId>
                <artifactId>cloud-core-quarkus-bom-publish</artifactId>
                <version>10.4.2<!-- >= 10.4.2 --></version>
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

Two separate artifacts exist, one per framework:

- Spring → [`httproutes-generator-spring-maven-plugin`](https://github.com/Netcracker/qubership-core-java-libs/blob/main/core-maven-plugins/httproutes-generator-spring-maven-plugin/README.md)
- Quarkus → [`httproutes-generator-maven-plugin`](https://github.com/Netcracker/qubership-core-java-libs/blob/main/core-maven-plugins/httproutes-generator-maven-plugin/README.md)

Each plugin fails the build if it detects the *other* framework's markers with none of its own —
pick the one matching your service.

If your Java service uses route-registration annotations:
1. Add the plugin matching your framework.
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

Use plugin coordinates `com.netcracker.cloud.plugins:httproutes-generator-spring-maven-plugin`
(Spring) or `com.netcracker.cloud.plugins:httproutes-generator-maven-plugin` (Quarkus), and pick
the latest release, `1.2.2`, and never a lower version. **Always set
`<backendRefVal>` explicitly** to `{{ .Values.SERVICE_NAME }}`. Do not rely on
the plugin default.

```xml
<build>
    <plugins>
        <plugin>
            <groupId>com.netcracker.cloud.plugins</groupId>
            <artifactId>httproutes-generator-spring-maven-plugin</artifactId> <!-- or httproutes-generator-maven-plugin for Quarkus -->
            <version>1.2.2</version>
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
                <backendRefVal><!-- use detected backendRefName from Step 1 — do not rely on the plugin default --></backendRefVal>
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

The plugin outputs a gateway-group route
(`{{ .Values.SERVICE_NAME }}-java-annotations`, parented to `public-gateway`, `private-gateway`
and `internal-gateway-service` by default — override via chart value
`HTTPROUTES_GENERATOR_GATEWAYS` only if you need a different set) and, only if a facade route
needs a rewrite or a longer-than-default timeout, a facade route
(`{{ .Values.SERVICE_NAME }}-java-annotations-facade`) — split into multiple CRs if a route
exceeds the Gateway API 16-rule limit. It never generates a per-type
`-public`/`-private`/`-internal` HTTPRoute, never generates `RegularExpression` matches, and never
generates an `AuthorizationPolicy` — see [Step 2.5](#step-25--public-gateway-access-control).

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
3. The skill scans Go and Java files, detects route definitions, reduces them to
   `PathPrefix` roots, and outputs HTTPRoute CRs. Legacy Public routes collapse
   into one gateway-group CR parented to `public-gateway`, `private-gateway` and
   `internal-gateway-service`. Private and Internal routes stay on a separate CR
   (`<name>-source-code-routes-nonpublic`) without `public-gateway` until effective
   access is checked (see [allowed-and-forbidden-routes.md](../allowed-and-forbidden-routes.md)).
   A group with more than 16 output rules is split into numbered objects
   `<name>-1`, `<name>-2`, and so on, in the same file. `Forbidden: true` /
   `allowed: false` routes are never emitted.
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

## Step 2.5 — Public Gateway Access Control

Per [allowed-and-forbidden-routes.md](../allowed-and-forbidden-routes.md), a route that was already
public is registered on `public-gateway`, `private-gateway` and `internal-gateway-service`. A
formerly private or internal route stays off `public-gateway` until you have checked the
AuthorizationPolicies that target it. Legacy `allowed: false` / `deny: true` rules are omitted,
not registered. Do not forbid those routes on the private gateway. A deny is drafted onto the
public gateway only when its source gateway was public.

Cloud-Core ships a deny-unless-permit `AuthorizationPolicy` for `public-gateway` (`action: ALLOW`
with no `rules`). A project-level ALLOW can still match a route you attach. **Write the service
policy by hand** — only you know which endpoints are meant to be public. You need no policy of
your own only after that check shows no ALLOW matches the service.

The skill
[`public-gateway-authz-policy`](../../../agent-packages/public-gateway-authz-policy/.apm/skills/public-gateway-authz-policy/SKILL.md)
collects candidate paths and prints a template. A recommendation still writes
`.mesh-migration/reports/public-gateway-authz-policy.yaml` and, on a standalone run, can append
`.mesh-migration/` to `.gitignore`. It creates
`<chart>/templates/public-gateway-authorization-policy.yaml` only when you explicitly ask for a
draft, and that file still needs review before merging.

Example hand-written policy (see [crs-example.yaml](../crs-example.yaml) for a fuller walkthrough).
Paths are exact unless they end in `*`, so list `/x` and `/x/*`. To block a path that another
policy allows (the legacy `deny: true`), use a separate `action: DENY` policy:

```yaml
{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: '{{ .Values.SERVICE_NAME }}-public-gateway-allow'
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: public-gateway
  action: ALLOW
  rules:
    - to:
        - operation:
            paths:
              - /api/v1/my-service/public
              - /api/v1/my-service/public/*
              - /api/v1/my-service/health
{{- end }}
```

---

## Step 2.6 — Verify All Routes Are Wrapped in Istio Conditionals

Every HTTPRoute CR in your Helm templates must be wrapped in a conditional guard so it only renders when Istio is the active mesh. This prevents Gateway API resources from being applied to clusters running Core mesh.

### Required pattern

```yaml
{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-service-source-code-routes
spec:
  ...
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-service-source-code-facade-routes
  ...
{{- end }}
```

### Validation

To verify, render with Istio disabled and confirm no Istio / Gateway API objects are present.
The Core render has to succeed before the search. An empty pipe from a failed `helm template` is
not a pass.

```bash
core_render=$(mktemp)
if helm template . --set SERVICE_MESH_TYPE=Core > "$core_render"; then
  grep -E '^apiVersion: (gateway\.networking\.k8s\.io|networking\.istio\.io|security\.istio\.io|extensions\.istio\.io)/' "$core_render"
  # Expected: no matches. grep exit code 1 is the pass.
else
  echo "FAIL: Core render failed"
fi
rm -f "$core_render"
```

---

## Step 2.7 — Detect Duplicate HTTPRoute Rules

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

## Step 2.8 — Flag Imperative Control-Plane Calls

Routes and other mesh config can also be pushed to the Cloud-Core Control-Plane
**imperatively** — for example a `curl`/`wget` against the control-plane REST API
from a shell script, a Helm hook, a Kubernetes `Job`, an init-container, or a
lifecycle command. Those calls bypass declarative CRs, the route-registration
libraries, and the `SERVICE_MESH_TYPE` guard, so Steps 1–2.7 never convert them.
Under Istio the control-plane no longer routes this traffic, so each call needs a
human decision: drop it, or replace it with an equivalent HTTPRoute.

Do **not** auto-edit these scripts. Scan and flag only.

Search the service repository (chart plus scripts) for shell that talks to the
control-plane — standalone `*.sh` / `*.bash` files, and `command:` / `args:` shell
in `Job`, `CronJob`, init-containers, container `lifecycle` hooks, and Helm hook
resources:

```bash
grep -rniE 'control-plane|controlplane|/api/v[0-9]+/(routes|control-plane)' \
  --include='*.sh' --include='*.bash' --include='*.yaml' --include='*.yml' .
```

Treat any `curl`/`wget`/HTTP client call whose target host or path references the
control-plane (for example `control-plane:8080`, `/api/v3/routes/...`,
registration or blue-green endpoints) as a hit. When in doubt, flag it — a false
positive costs a glance; a missed call silently breaks routing after cutover.

For each hit, record a manual review item with:

- the file (and line) of the call,
- the control-plane endpoint and HTTP method,
- the action you will take: confirm an HTTPRoute already covers the route and
  remove the call, or keep Core-mode compatibility by gating the call.

Gate forms:

- **Helm-templated manifest** (`Job`, `CronJob`, init-container, `lifecycle`
  hook, hook resource) — wrap in
  `{{- if eq .Values.SERVICE_MESH_TYPE "Core" }}` … `{{- end }}`.
- **Standalone shell script** — exit early unless the mesh type is Core, for
  example `[ "$SERVICE_MESH_TYPE" = "Core" ] || exit 0`. The script's container
  must receive `SERVICE_MESH_TYPE`; Deployment env wiring does not cover `Job` or
  hook pods.

---

## Upgrading an in-progress migration

Skill versions before the public-gateway policy step numbered validation as 2.5, 2.6, and 2.7.
Those prompts now land on policy authoring if you follow the numbers literally.

| Old step | What it was | Current step |
|---|---|---|
| 2.5 | Istio guards and render checks | 2.6 |
| 2.6 | Duplicate HTTPRoute rules | 2.7 |
| 2.7 | Imperative control-plane calls | 2.8 |

"Rerun the validation" starts at current Step 2.6 (`istio-migration-validate`), not at Step 2.5.

Review these effects before deployment and keep them in the merge description:

- Gateway attachment is wider for routes that were already public: one HTTPRoute that used to
  parent only `public-gateway` now expects `public-gateway`, `private-gateway`, and
  `internal-gateway-service`. Formerly non-public routes stay off `public-gateway` until
  AuthorizationPolicies that target it have been checked. A rerun does not silently rewrite an
  old parent list when the file has consumer edits; use `reconcileParents: true` or edit the
  parents by hand.
- Path regexes are cut to a prefix at the first path variable and are not emitted. A cut is
  called out when it conflicts with another rule. Legacy `allowed: false` and `deny: true` rules
  are omitted instead of registered as 404 routes. The forbidden-route comment asks whether a
  remaining rule and the public policy cover the path.
- Service owners commit generated routes. That includes `source-code-httproutes.yaml` from
  `httproute-from-code` and the Maven plugin's `annotations-httproutes.yaml`. Rebuild or rerun
  the generator when route code or annotations change, then commit the output.

## Final Checklist

Before raising a PR, verify all of the following:

- [ ] [`core-mesh-crs-to-istio`](../../../agent-packages/core-mesh-crs-to-istio/.apm/skills/core-mesh-crs-to-istio/SKILL.md) skill run: existing mesh CRs converted to HTTPRoute CRs
- [ ] Flagged features from Step 1.1 manually resolved
- [ ] New mesh-aware libraries replace old route-posting libraries
- [ ] `SERVICE_MESH_TYPE` wired in Helm values / Deployment (default `Core`; environments set `Istio`)
- [ ] Maven plugin added and local build passes (Java only)
- [ ] [`httproute-from-code`](../../../agent-packages/httproute-from-code/.apm/skills/httproute-from-code/SKILL.md) skill run: route registration code converted to HTTPRoute CRs
- [ ] Public-gateway `AuthorizationPolicy` written by hand (a generated draft is reviewed before merging), or confirmed unnecessary
- [ ] All HTTPRoute CRs wrapped in a `{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}` block
- [ ] HTTPRoutes scanned for duplicate rules (same parent + equal match); duplicates resolved or flagged for review
- [ ] Imperative control-plane API calls in scripts and manifests flagged for review (or removed / gated for Core)

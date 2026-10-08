---
name: mesh-build-wiring
description: >
  Wire a service's build and deployment for mesh-type awareness: switch to
  mesh-aware route-registration libraries (Java Spring/Quarkus, Go), set the
  SERVICE_MESH_TYPE environment variable in Helm values and Deployments, and add
  the httproutes-generator Maven plugin for Java services. Use when asked to
  prepare a service's dependencies and env for the Core Mesh to Istio migration,
  or as a sub-skill of core-mesh-to-istio-migration.
---

# Mesh-type-aware build and deployment wiring

## Contract

### Inputs

| Input | Type | Required | Notes |
|---|---|---|---|
| `codePath` | path | yes | Service source root (contains `pom.xml` / `go.mod`) |
| `chartPath` | path | yes | Helm chart with `values.yaml` / Deployment templates |
| `language` | `go \| java \| both` | yes | Languages actually present in the repo |
| `backendRefName` | string | for Java | `<backendRefVal>` for the Maven plugin; Helm expressions allowed |
| `backendRefPort` | integer | for Java | `<servicePort>` for the Maven plugin |
| `routeLabels` | map | for Java | `<labels>` for the Maven plugin |
| `interactive` | bool | no | `true` only when a user invokes the skill directly; orchestrators and sub-agent wrappers pass `false` |
| `resolutions` | map `<unresolved id>: <answer>` | no | Answers to a previous run's `unresolved:` entries |

With `interactive: false` (the default for orchestrated and sub-agent runs),
never ask the user: every blocking question becomes an `unresolved:` entry.
Skip only the work that depends on the answer, **continue with every other
step**, and set `status: partial` when writing the final report. With
`interactive: true`, ask blocking questions in chat — including missing
required inputs (propose the defaults `{{ .Values.DEPLOYMENT_RESOURCE_NAME }}`
/ `8080` for the backend reference).

### Outputs

In addition to the chat summary, write a machine-readable report to
`.mesh-migration/reports/mesh-build-wiring.yaml` (create the directory, and ensure
`.mesh-migration/` is listed in the repo's `.gitignore` — reports are working
files, never committed; the orchestrator handles both in orchestrated runs):

```yaml
reportSchema: 1
skill: mesh-build-wiring
status: complete            # complete | partial | failed
generatedAt: <ISO-8601>
done:
  - <one line per applied change, e.g. "pom.xml: rest-libraries-bom bumped to 7.1.0">
skipped:
  - <one line per intentionally skipped item, with reason>
commandsRun:
  - command: <cmd>
    exitCode: <N>
forbiddenRouteErrors:       # verbatim plugin [ERROR] lines of Step 3.3; empty when none
  - <line>
blockedBy: null             # java-forbidden-routes when the user chose split-controllers
unresolved: []              # blocking user decisions, each {id, question, options, default}
needsReview:
  - <one line per item requiring human review>
```

`status: partial` with an empty `unresolved:` and `blockedBy: java-forbidden-routes`
means the user chose to refactor controllers: the migration must stop until they
have, and resume from Step 2.3.

Consumers must ignore unknown report fields. A consumer that sees a
`reportSchema` newer than its own documentation must stop and report a contract
mismatch instead of guessing field meanings.

On an unrecoverable error (required build command fails, file cannot be
written), stop, set `status: failed`, and put the error under `needsReview:`.

### Side effects

Edits `pom.xml` / `go.mod`, `values.yaml` / `values.schema.json`, and
Deployment templates. Runs `go mod tidy` / `mvn -q clean process-classes` when
available, writing the Maven log to `.mesh-migration/work/`. Writes the report
file. Edits Java sources **only** to add `@ForbiddenRoute` annotations and their
imports, and only after the user chose `forbidden-route-annotations` (Step 3.3).
Nothing else.

---

## Step 1 — Switch to mesh-type-aware route-registration libraries

Apply only to languages actually present in the repo.

### Java

**Idempotency check:** for each dependency below, check whether the current
version already satisfies the minimum. If yes, record under `done:` ("already
compliant") and skip that dependency.

Minimum versions — older ones must be bumped, including versions that satisfied
an earlier migration (`7.1.0`, `12.0.2`, `9.1.0`). They bring
`com.netcracker.cloud:route-registration-common` `7.5.2`, which contains
`@ForbiddenRoute`; plugin `1.1.5` (Step 3) needs it for services with forbidden
routes.

| Framework | Explicit dependency | BOM (preferred with `dependencyManagement`) |
|---|---|---|
| Spring | `com.netcracker.cloud:route-registration-webclient` or `com.netcracker.cloud:route-registration-resttemplate` `>= 7.5.2` | `com.netcracker.cloud:cloud-core-java-bom` `>= 12.2.5`, or `com.netcracker.cloud:rest-libraries-bom` `>= 7.5.2` |
| Quarkus | `com.netcracker.cloud.quarkus:routes-registrator` `>= 10.3.2` | `com.netcracker.cloud:cloud-core-quarkus-bom-publish` `>= 10.3.2` |

- **Spring** (`spring-boot-starter-*` detected in `pom.xml`): replace old
  route-posting dependencies with the webclient or resttemplate artifact. If the
  project uses `dependencyManagement`, upgrade (or add) the BOM instead of adding
  duplicate explicit dependency versions.
- **Quarkus** (`quarkus-*` detected in `pom.xml`): replace or add
  `routes-registrator`; with `dependencyManagement`, upgrade (or add) the BOM
  instead.
- Use exactly these minimums (or a newer version already in the `pom.xml`). If
  the build later fails because a version cannot be resolved (`Could not find
  artifact` / `Could not resolve dependencies`), do not try other versions:
  `status: failed` with a `needsReview:` entry naming the artifact and version.
- If the choice between webclient and resttemplate variants is ambiguous, do
  not guess: check the `resolutions` input for id
  `java-registration-artifact`; if absent, with `interactive: true` ask the
  user, otherwise add an `unresolved:` entry (id `java-registration-artifact`,
  options `[route-registration-webclient, route-registration-resttemplate]`)
  and skip **only this dependency swap** — Steps 2 and 3 still run. Set
  `status: partial` when writing the final report.

### Go

**Idempotency check:** read `go.mod` before making any changes.

- In `go.mod`, find `github.com/netcracker/qubership-core-lib-go-rest-utils/v2`.
- If present with version `>= v2.5.0` → record under `done:` ("already compliant").
- If present with a lower version → bump to at least `v2.5.0`, run
  `go mod tidy`, and record the exit code under `commandsRun:`. If it exits
  non-zero → `status: failed` per the contract.
- If absent → do not add it automatically; add a `needsReview:` entry:
  "Go route-registration dependency not found — confirm the service does not
  register routes in code."
- If the repo contains a `go.work` file (Go workspace), add a `needsReview:`
  entry: "Go workspace (`go.work`) detected — multi-module dependency bumps are
  out of scope for this skill and require manual handling."
- If multiple modules import `rest-utils` at different versions, add a
  `needsReview:` entry for each conflicting module.

## Step 2 — Wire `SERVICE_MESH_TYPE` into the Deployment

**Ownership:** this skill owns Deployment / pod `env:` wiring. Chart
`values.yaml` / `values.schema.json` may already contain `SERVICE_MESH_TYPE`
(same property contract below) — treat those as ensure-only.

**Idempotency check:** before editing any file, check whether the target is
already correct. If yes, record under `done:` ("already present") and skip that
file.

All services that use route registration libraries must receive
`SERVICE_MESH_TYPE`. Default Helm value is `Core` (Istio not required yet);
environments override to `Istio` when ready.

| Target | Action |
| --- | --- |
| `values.yaml` | Ensure `SERVICE_MESH_TYPE: "Core"` (add if missing; do not overwrite a deliberate non-default). |
| `values.schema.json` (if present) | Ensure the property entry below under `properties`, a root `"examples"` entry `{"SERVICE_MESH_TYPE": "Core"}`, and root `"additionalProperties": true`. |
| Helm `Deployment` template | Ensure `env:` has `SERVICE_MESH_TYPE` with `value: '{{ .Values.SERVICE_MESH_TYPE }}'`. |
| Plain Kubernetes `Deployment` | Add `- name: SERVICE_MESH_TYPE` with `value: Core`, or template it if Helm-rendered. |

Exact `values.schema.json` property entry:

```json
    "SERVICE_MESH_TYPE": {
      "$id": "#/properties/SERVICE_MESH_TYPE",
      "type": "string",
      "title": "The SERVICE_MESH_TYPE schema",
      "description": "Service mesh type. Use `Core` for Cloud Core Mesh or `Istio` for Istio Ambient Mesh.",
      "enum": ["Istio", "Core"],
      "default": "Core",
      "internal": true
    }
```

Record under `done:` the exact files edited. If multiple Deployments exist, list
each. If the desired runtime mesh for an environment is unclear, keep the default
`Core` and add a `needsReview:` entry telling the user where to set `Istio`.

## Step 3 — Add the Maven plugin (Java services only)

**Idempotency check:** if `httproutes-generator-maven-plugin` is already present
in `pom.xml` with version `>= 1.1.5`, record under `done:` ("already present")
and go straight to [Step 3.3](#step-33--build-and-detect-forbidden-routes) — the
forbidden-route check runs on every run. A lower version → bump it to `>= 1.1.5`,
record under `done:`, and continue with Step 3.2.

- **If no `pom.xml`** → record under `skipped:` ("No pom.xml found — Go-only
  service") and finish.
- **If the Java service does not use route-registration annotations** → record
  under `skipped:` with the reason and finish.
- **If `pom.xml` exists and annotations are used**, follow these sub-steps
  (from the [plugin README](https://github.com/Netcracker/qubership-core-java-libs/blob/main/core-maven-plugins/httproutes-generator-maven-plugin/README.md)):

### Step 3.1 — Add the plugin to `pom.xml`

- `<groupId>com.netcracker.cloud.plugins</groupId>`
- `<artifactId>httproutes-generator-maven-plugin</artifactId>`
- `<version>` must use the latest available release, but never lower than
  `1.1.5` (`>= 1.1.5`). Older versions emit regex matches for paths with
  `{variables}`, which Istio cannot rewrite.
- `<goal>generate-routes</goal>`
- `<packages>` resolved from `src/main/java/...`. If ambiguous, set
  `com.example` and add a `needsReview:` entry.
- `<servicePort>` set to the `backendRefPort` input.
- `<outputFile>` pointing inside the chart templates directory, defaulting
  to `<chartPath>/templates/annotations-httproutes.yaml`.
- `<backendRefVal>` set to the `backendRefName` input, e.g.
  `<backendRefVal>{{ .Values.DEPLOYMENT_RESOURCE_NAME }}</backendRefVal>`.
  Always set it: the plugin default changed to `{{ .Values.SERVICE_NAME }}` in
  `1.1.5`.
- `<labels>` set to the `routeLabels` input, in Maven plugin label format:
  `<labels><label><key>my/special-key</key><value>value1</value></label></labels>`.
  Do not invent values.
- Do **not** add `<autoGenerateAuthorizationPolicies>` — only the user's answer in
  Step 3.3 may add it.

### Step 3.2 — Confirm `<outputFile>`

It must point inside the Helm chart templates directory. This file must be
committed to the branch.

### Step 3.3 — Build and detect forbidden routes

Plugin `1.1.5` cuts every gateway path before its first `{variable}` (Istio has
no regex routes) and checks the result against legacy behavior on the public
and private gateways. A path that legacy forbade (404) but Istio would route
**fails the build** until a DENY rule covers it.

1. If Maven is not available in the environment → record under `skipped:`
   ("mvn not available in environment") and add this `needsReview:` entry, then
   continue with Step 3.6:
   `Build locally with mvn clean process-classes. If httproutes-generator-maven-plugin fails with "route migration errors", rerun this migration from Step 2.3 — it asks how to fix forbidden routes: split controllers (recommended; gateway URLs of moved endpoints change), add @ForbiddenRoute, or set autoGenerateAuthorizationPolicies.`
2. Run, and record the command and exit code under `commandsRun:`:

   ```bash
   mkdir -p .mesh-migration/work
   mvn -B -q clean process-classes > .mesh-migration/work/mvn-process-classes.log 2>&1; echo "exit=$?"
   ```

3. Exit code `0` → no forbidden-route problems; continue with Step 3.6.
4. Non-zero exit code **without** the line `route migration errors, see log` in the
   log → `status: failed` per the contract (copy the first `[ERROR]` lines into
   `needsReview:`).
5. Non-zero exit code **with** `route migration errors, see log` → list the
   plugin errors and copy them verbatim into `forbiddenRouteErrors:`:

   ```bash
   grep -E '\[ERROR\] ' .mesh-migration/work/mvn-process-classes.log \
     | grep -E "add @ForbiddenRoute|can't express it|@ForbiddenRoute forbids|@ForbiddenRoute of"
   ```

   Classify each line by its text:

   | Type | The line contains | Meaning |
   |---|---|---|
   | E1 | `is forbidden by legacy, as its route type is narrower, but Istio routes it by PathPrefix` | an endpoint with a narrower route type lies below a wider controller's prefix |
   | E2 | `is not routed by legacy, but Istio routes it by PathPrefix` | the cut exposes a path legacy never routed |
   | E3 | `an AuthorizationPolicy can't express it: variables must take up whole path segments` | a partial variable segment (`/{name}.txt`) — only changing the path fixes it |
   | E4 | `@ForbiddenRoute forbids` … `where a route with this gateway path is exposed` | an existing `@ForbiddenRoute` contradicts a route |
   | E5 | `@ForbiddenRoute of` … `must list PUBLIC and/or PRIVATE` | an existing `@ForbiddenRoute` lists a wrong type |

   - Any E4 / E5 line → `status: failed`; add each line to `needsReview:` with
     "fix the existing @ForbiddenRoute annotation". Stop.
   - E1 / E2 / E3 lines → **stop this step and ask** (Step 3.4). Do not
     continue to Step 3.6: the plugin wrote no output file.

### Step 3.4 — Ask how to fix forbidden routes

Check the `resolutions` input for id `java-forbidden-routes`. When absent:
with `interactive: true`, ask the user in chat and wait; with
`interactive: false`, add this `unresolved:` entry, skip Steps 3.5–3.7, and
finish with `status: partial`:

```yaml
- id: java-forbidden-routes
  question: <the text below, filled in>
  options: [split-controllers, forbidden-route-annotations, auto-generate-authorization-policies]
  default: split-controllers
```

Question text — fill `<…>`, keep the rest verbatim:

```text
httproutes-generator-maven-plugin stopped the build: <N> gateway path(s) were forbidden (404) in legacy Cloud-Core Mesh, but Istio would route them.

<every E1 / E2 / E3 line, verbatim, one per line>

Why: Istio has no regex routes. Every gateway path is cut before its first {variable} and matched as a PathPrefix, and a class-level PathPrefix routes everything below it — so an endpoint with a narrower route type inside a wider controller, or a path the cut exposes, becomes reachable on the public / private gateway.

How should they be fixed?

1. split-controllers (Recommended) — move the narrower endpoints into their own controllers, with a separate class-level gateway prefix per exposure level (for example a public controller on /api/v1/<service>/resource and a private controller on /api/v1/<service>/internal/resource). Each PathPrefix then has a single route type, so routing alone keeps them apart: no DENY policies are needed and legacy 404s stay 404.
   ⚠ BACKWARD COMPATIBILITY BREAK: the gateway URLs of the moved endpoints change — every client that calls them must switch to the new paths. The migration stops here: refactor the controllers, then rerun the migration from Step 2.3.
2. forbidden-route-annotations — add @ForbiddenRoute(...) to each element the errors name. URLs stay unchanged; the plugin generates an AuthorizationPolicy DENY rule per forbidden path, and a denied request gets 403 "RBAC: access denied" instead of the legacy 404.
3. auto-generate-authorization-policies — set <autoGenerateAuthorizationPolicies>true</autoGenerateAuthorizationPolicies> in the plugin configuration, and the plugin generates every missing DENY rule itself. Use it when adding @ForbiddenRoute to each element is too complex. Same 403 change, and the forbidden paths are no longer visible in the code.
```

When there are E3 lines, append:
`<n> error(s) say "variables must take up whole path segments": DENY rules cannot express those paths, so only changing them (option 1) fixes them, whichever option you choose.`

### Step 3.5 — Apply the answer

**`split-controllers`** — do not edit code.

- Add one `needsReview:` entry per E1 / E2 / E3 line: `<path from the line>: move
  it to a controller with its own class-level gateway prefix (BWC break: its URL
  changes), then rerun the migration from Step 2.3.`
- Set `blockedBy: java-forbidden-routes` and `status: partial` (with an empty
  `unresolved:`). Skip Steps 3.6–3.7 and finish.

**`forbidden-route-annotations`** — for each E1 / E2 line:

1. Read the path `P` after `to the element mapped to ` and the types inside
   `add @ForbiddenRoute({…})` (for example `PUBLIC, PRIVATE`).
2. Find the element mapped to `P`. For every class under `src/main/java` with a
   route mapping annotation:
   - class gateway path = the class-level `@GatewayRequestMapping` value, else
     its `@RequestMapping` / JAX-RS `@Path` value, else empty;
   - method gateway path = the method-level `@GatewayRequestMapping` value, else
     its `@RequestMapping` / `@GetMapping` / `@PostMapping` / `@PutMapping` /
     `@DeleteMapping` / `@PatchMapping` / `@Path` value;
   - element path = the class gateway path joined with the method gateway path
     by one `/`.

   Compare with every `{name}` replaced by `{*}`. `P` equal to a class gateway
   path → the element is the **class**; otherwise the **method** whose element
   path equals `P`.
3. Add `@ForbiddenRoute({RouteType.<TYPE>, …})` — the types from the error, each
   prefixed with `RouteType.` — on its own line directly above the element's
   first annotation. Add the imports when missing:
   `import com.netcracker.cloud.routesregistration.common.annotation.ForbiddenRoute;`
   and
   `import com.netcracker.cloud.routesregistration.common.gateway.route.RouteType;`
   (if the file already imports another `RouteType`, write the annotation with
   fully qualified names instead).
4. No element found — typical for E2 when the cut prefix maps to no class or
   method (for example `/api` cut from `/api/{version}/...`) → edit nothing for
   that line. After all lines: if any were unmapped, ask (or add `unresolved:`)
   id `java-forbidden-routes-unmapped`, options
   `[auto-generate-authorization-policies, split-controllers]`, default
   `split-controllers`, question:
   `@ForbiddenRoute cannot be added for <paths>: no class or method is mapped to them. Generate their DENY rules with autoGenerateAuthorizationPolicies, or split the controllers (BWC break: URLs change)?`

Record each annotated `file:line` under `done:`. E3 lines are handled like
`split-controllers` (needsReview + `blockedBy`), whatever the answer.

**`auto-generate-authorization-policies`** — add
`<autoGenerateAuthorizationPolicies>true</autoGenerateAuthorizationPolicies>` to the
plugin `<configuration>`; record under `done:`.

Then rebuild with the command of Step 3.3 and classify again — at most two
rebuilds. New E1 / E2 lines after annotating → annotate them too. Still failing
after the second rebuild → `status: failed`. Once the build is green:

- confirm `<outputFile>` contains `kind: AuthorizationPolicy`, else add a
  `needsReview:` entry;
- add these three `needsReview:` entries:
  - `AuthorizationPolicy path templates ({*}, {**}) require Istio >= 1.22.`
  - `meshConfig.pathNormalization.normalization must be MERGE_SLASHES or stronger — otherwise %2F, .. and // bypass the DENY rules.`
  - `Denied requests now get 403 "RBAC: access denied" instead of the legacy 404.`

### Step 3.6 — Commit the generated `<outputFile>`

Commit it to the branch. Remind the user:

> The plugin generates the output file at compile time. Every time route
> annotations change, run `mvn clean compile` locally and commit the updated
> output file before raising a PR.

### Step 3.7 — Record

Record the selected plugin version, the forbidden-route answer (if any), and the
committed file path under `done:`.

---

## Output

Write the contract report file (see [Contract → Outputs](#outputs)), then print
a short chat summary: files edited, commands run with exit codes, skipped items,
and every `needsReview:` entry.

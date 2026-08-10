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
never ask the user: every blocking question becomes an `unresolved:` entry and
the run finishes with `status: partial`. With `interactive: true`, ask blocking
questions in chat — including missing required inputs (propose the defaults
`{{ .Values.DEPLOYMENT_RESOURCE_NAME }}` / `8080` for the backend reference).

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
unresolved: []              # blocking user decisions, each {id, question, options, default}
needsReview:
  - <one line per item requiring human review>
```

Consumers must ignore unknown report fields. A consumer that sees a
`reportSchema` newer than its own documentation must stop and report a contract
mismatch instead of guessing field meanings.

On an unrecoverable error (required build command fails, file cannot be
written), stop, set `status: failed`, and put the error under `needsReview:`.

### Side effects

Edits `pom.xml` / `go.mod`, `values.yaml` / `values.schema.json`, and
Deployment templates. Runs `go mod tidy` / `mvn -q clean process-classes` when
available. Writes the report file. Nothing else.

---

## Step 1 — Switch to mesh-type-aware route-registration libraries

Apply only to languages actually present in the repo.

### Java

**Idempotency check:** for each dependency below, check whether the current
version already satisfies the minimum. If yes, record under `done:` ("already
compliant") and skip that dependency.

- **Spring** (`spring-boot-starter-*` detected in `pom.xml`):
  - Replace old route-posting dependencies with either
    `com.netcracker.cloud:route-registration-webclient` or
    `com.netcracker.cloud:route-registration-resttemplate` at version `>= 7.1.0`.
  - If the project uses `dependencyManagement`, prefer an existing or upgraded
    `com.netcracker.cloud:rest-libraries-bom` at version `>= 7.1.0`, or
    `com.netcracker.cloud:cloud-core-java-bom` at version `>= 12.0.2`, instead
    of adding duplicate explicit dependency versions.
- **Quarkus** (`quarkus-*` detected in `pom.xml`):
  - Replace or add `com.netcracker.cloud.quarkus:routes-registrator` at version
    `>= 9.1.0`.
  - If the project uses `dependencyManagement`, prefer an existing or upgraded
    `com.netcracker.cloud:cloud-core-quarkus-bom-publish` at version `>= 9.1.0`
    instead of adding duplicate explicit dependency versions.
- If the choice between webclient and resttemplate variants is ambiguous, do
  not guess: check the `resolutions` input for id
  `java-registration-artifact`; if absent, with `interactive: true` ask the
  user, otherwise add an `unresolved:` entry (id `java-registration-artifact`,
  options `[route-registration-webclient, route-registration-resttemplate]`),
  skip the dependency change, and finish with `status: partial`.

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

## Step 2 — Set the `SERVICE_MESH_TYPE` environment variable

**Idempotency check:** before editing any file, check whether `SERVICE_MESH_TYPE`
is already present with the correct value and schema entry. If yes, record
under `done:` ("already present") for that file and skip it.

All services that use route registration libraries must receive
`SERVICE_MESH_TYPE`. By default, set Helm values to `Core` for compatibility with
environments where Istio is not installed yet; deployments can override the value
to `Istio` when migrating an environment.

| Deployment source                          | Action                                                                                           |
| ------------------------------------------ | ------------------------------------------------------------------------------------------------ |
| Helm values drive a `Deployment` template  | Ensure `values.yaml` has `SERVICE_MESH_TYPE: "Core"` and the Deployment `env:` uses `value: '{{ .Values.SERVICE_MESH_TYPE }}'`. |
| `values.schema.json` exists                | Ensure `SERVICE_MESH_TYPE` has a full schema entry: `"type": "string"`, `"enum": ["Istio", "Core"]`, `"default": "Core"`, `"$id": "#/properties/SERVICE_MESH_TYPE"`, `"internal": true`, exact `"description": "Service mesh type. Use `Core` for Cloud Core Mesh or `Istio` for Istio Ambient Mesh."`, and an entry in the root-level `"examples"` array (`{"SERVICE_MESH_TYPE": "Core"}`). Also confirm `"additionalProperties": true` is set at the root. |
| Plain Kubernetes `Deployment` manifest     | Add `- name: SERVICE_MESH_TYPE` with `value: Core`, or template it if the manifest is Helm-rendered. |

Record under `done:` the exact files edited. If multiple Deployments exist, list
each. If the desired runtime mesh for an environment is unclear, keep the default
`Core` and add a `needsReview:` entry telling the user where to set `Istio`.

## Step 3 — Add the Maven plugin (Java services only)

**Idempotency check:** if `httproutes-generator-maven-plugin` is already present
in `pom.xml`, record under `done:` ("already present") and finish.

- **If no `pom.xml`** → record under `skipped:` ("No pom.xml found — Go-only
  service") and finish.
- **If the Java service does not use route-registration annotations** → record
  under `skipped:` with the reason and finish.
- **If `pom.xml` exists and annotations are used**, follow these five sub-steps
  (from the [plugin README](https://github.com/Netcracker/qubership-core-java-libs/blob/main/core-maven-plugins/httproutes-generator-maven-plugin/README.md)):
  1. **Add plugin to `pom.xml`** with the following configuration:
     - `<groupId>com.netcracker.cloud.plugins</groupId>`
     - `<artifactId>httproutes-generator-maven-plugin</artifactId>`
     - `<version>` must use the latest available release, but never lower than
       `1.0.2` (`>= 1.0.2`).
     - `<goal>generate-routes</goal>`
     - `<packages>` resolved from `src/main/java/...`. If ambiguous, set
       `com.example` and add a `needsReview:` entry.
     - `<servicePort>` set to the `backendRefPort` input.
     - `<outputFile>` pointing inside the chart templates directory, defaulting
       to `<chartPath>/templates/annotations-httproutes.yaml`.
     - `<backendRefVal>` set to the `backendRefName` input, e.g.
       `<backendRefVal>{{ .Values.DEPLOYMENT_RESOURCE_NAME }}</backendRefVal>`.
     - `<labels>` set to the `routeLabels` input, in Maven plugin label format:
       `<labels><label><key>my/special-key</key><value>value1</value></label></labels>`.
       Do not invent values.
  2. **Confirm `<outputFile>`** is set to a path inside the Helm chart templates
     directory. This file must be committed to the branch.
  3. **Build the project** to generate the output file. Run
     `mvn -q clean process-classes` if Maven is available in the environment and
     record the exit code under `commandsRun:`. If Maven is not available,
     record under `skipped:` ("mvn not available in environment") and continue.
     If Maven is available but exits non-zero → `status: failed` per the
     contract.
  4. **Commit the generated `<outputFile>`** to the branch. Remind the user:
     > The plugin generates the output file at compile time. Every time route
     > annotations change, run `mvn clean compile` locally and commit the updated
     > output file before raising a PR.
  5. Record the selected plugin version and committed file path under `done:`.

---

## Output

Write the contract report file (see [Contract → Outputs](#outputs)), then print
a short chat summary: files edited, commands run with exit codes, skipped items,
and every `needsReview:` entry.

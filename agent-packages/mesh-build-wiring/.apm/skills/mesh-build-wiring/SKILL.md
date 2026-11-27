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
required inputs (propose the defaults `{{ .Values.SERVICE_NAME }}`
/ `8080` for the backend reference).

### Outputs

In addition to the chat summary, write a machine-readable report to
`.mesh-migration/reports/mesh-build-wiring.yaml` (create the directory, and ensure
`.mesh-migration/` is listed in the repo's `.gitignore` — reports are working
files, never committed; the orchestrator handles both in orchestrated runs):

```yaml
reportSchema: 1
skill: mesh-build-wiring
chartPath: <chartPath>
status: complete            # complete | partial | failed
generatedAt: <ISO-8601>
versions:
  routesRegistrator: <version applied or already present, or null>
  httpRoutesGeneratorPlugin: <version applied or already present, or null>
done:
  - <one line per applied change, e.g. "pom.xml: rest-libraries-bom bumped to 7.6.2">
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

**Versions:** when adding or bumping, set exactly the minimum version stated in this section
(never invent or guess a newer one — you may have no registry access); leave an already higher
version as is.

**Quarkus `routes-registrator` floor** (the only place this skill states it): `10.4.2`, the latest
release (`cloud-core-quarkus-10.4.2`). Use it with or without `httproutes-generator-maven-plugin`.
Plugin `1.2.2` is built against this version, so a 9.1.x BOM does not meet the floor. Step 1 and
Step 3 both mean this floor. Record the version you left in the file under
`versions.routesRegistrator`. Do not repeat the numbers in the plugin branches.

### Java

If the build uses Gradle (`build.gradle` / `build.gradle.kts`) and no `pom.xml`, do not edit it: add a
`needsReview:` entry "Gradle build detected — apply the same dependency minimums by hand; the Maven
plugins do not apply" and skip the rest of Java handling (Step 2 still runs).

**Idempotency check:** for each dependency below, check whether the current
version already satisfies the minimum. If yes, record under `done:` ("already
compliant") and skip that dependency.

- **Spring** (`spring-boot-starter-*` detected in `pom.xml`):
  - Replace old route-posting dependencies with either
    `com.netcracker.cloud:route-registration-webclient` or
    `com.netcracker.cloud:route-registration-resttemplate` at version `>= 7.6.2`.
  - If the project uses `dependencyManagement`, prefer an existing or upgraded
    `com.netcracker.cloud:rest-libraries-bom` at version `>= 7.6.2`, or
    `com.netcracker.cloud:cloud-core-java-bom` at version `>= 12.3.2`, instead
    of adding duplicate explicit dependency versions.
- **Quarkus** (`quarkus-*` detected in `pom.xml`):
  - Replace or add `com.netcracker.cloud.quarkus:routes-registrator` at the
    Quarkus `routes-registrator` floor defined above, whether or not the service
    uses `httproutes-generator-maven-plugin` (see
    [Step 3](#step-3--add-the-maven-plugin-java-services-only)).
  - If the project uses `dependencyManagement`, prefer an existing or upgraded
    `com.netcracker.cloud:cloud-core-quarkus-bom-publish` that yields at least
    that same floor, instead of adding a duplicate explicit dependency version.
- Choosing webclient vs resttemplate: keep the variant already in `pom.xml`; otherwise grep the
  sources for `WebClient` vs `RestTemplate` imports and pick the one that is used. If the choice is
  still ambiguous, do not guess: check the `resolutions` input for id
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

Two separate artifacts exist, one per framework (see the
[Spring plugin README](https://github.com/Netcracker/qubership-core-java-libs/blob/main/core-maven-plugins/httproutes-generator-spring-maven-plugin/README.md)
and the
[Quarkus plugin README](https://github.com/Netcracker/qubership-core-java-libs/blob/main/core-maven-plugins/httproutes-generator-maven-plugin/README.md)):

| Framework | Artifact |
|---|---|
| Spring (`spring-boot-starter-*` detected in `pom.xml`) | `com.netcracker.cloud.plugins:httproutes-generator-spring-maven-plugin` |
| Quarkus (`quarkus-*` detected in `pom.xml`) | `com.netcracker.cloud.plugins:httproutes-generator-maven-plugin` |

Use the same framework detection as [Step 1](#step-1--switch-to-mesh-type-aware-route-registration-libraries).
If both markers are present (e.g. `quarkus-spring-web`) or neither is conclusive, do not guess:
check the `resolutions` input for id `java-framework`; if absent, with `interactive: true` ask the
user, otherwise add an `unresolved:` entry (id `java-framework`, options `[spring, quarkus]`) and
skip this step only — everything else still runs. Set `status: partial` when writing the final
report. Each plugin itself also fails the build (not this skill) if it detects the wrong
framework's markers with none of its own — see the README "Framework guard" section — so a wrong
guess here surfaces immediately at `mvn` time rather than silently.

**Idempotency check:** if the artifact matching the detected framework is already present in
`pom.xml` at version `>= 1.2.2`, record under `done:` ("already present"). For Quarkus
(`httproutes-generator-maven-plugin`), still verify `routes-registrator` against the floor defined
above and bump it when it is lower. If Step 1 or that bump changed `pom.xml`, run the generation
build in sub-step 3 and record it before finishing. If neither changed configuration, finish
without the build. If the *other* artifact is present instead (e.g. the old generic
`httproutes-generator-maven-plugin` on a Spring-only service), switch the `artifactId` to the one
matching the detected framework rather than adding a second plugin. If the right artifact is
present below `1.2.2`, bump the version.

- **If no `pom.xml`** → record under `skipped:` ("No pom.xml found — Go-only
  service") and finish.
- **If the Java service does not use route-registration annotations** → record
  under `skipped:` with the reason and finish. "Route-registration annotation" means exactly an
  import from `com.netcracker.cloud` of `Route`, `Gateway` or `GatewayRequestMapping`
  (grep `src/main/java` for `@Route`, `@Gateway`, `@GatewayRequestMapping`). Plain Spring
  `@RequestMapping` / `@GetMapping` do **not** count.
- **If `pom.xml` exists and annotations are used**, follow these five sub-steps:
  1. **Add the plugin matching the detected framework** to `pom.xml` with the following
     configuration:
     - `<groupId>com.netcracker.cloud.plugins</groupId>`
     - `<artifactId>httproutes-generator-spring-maven-plugin</artifactId>` (Spring) or
       `<artifactId>httproutes-generator-maven-plugin</artifactId>` (Quarkus).
     - `<version>` exactly `1.2.2` (keep a higher version if one is already there).
     - **Quarkus `routes-registrator`:** if the service uses the Quarkus Maven plugin, set
       `com.netcracker.cloud.quarkus:routes-registrator` to the plugin floor defined above when
       the current version is lower. Leave an already higher version as is. Record the version
       under `done:` and `versions.routesRegistrator`.
     - `<goal>generate-routes</goal>`
     - `<packages>` = the common base package of the classes under `src/main/java` that contain the
       annotations found above. If there is no single common base package, check `resolutions` for
       id `maven-plugin-packages`; if absent, with `interactive: true` ask the user, otherwise add
       an `unresolved:` entry (id `maven-plugin-packages`, question "Which Java packages contain
       your route annotations?"), skip this step only, and set `status: partial` — never write a
       placeholder like `com.example`.
     - `<servicePort>` set to the `backendRefPort` input.
     - `<outputFile>` pointing inside the chart templates directory, defaulting
       to `<chartPath>/templates/annotations-httproutes.yaml`.
     - `<backendRefVal>` **always set explicitly** to the `backendRefName` input, e.g.
       `<backendRefVal>{{ .Values.SERVICE_NAME }}</backendRefVal>` (the default when nothing was
       detected). Do not rely on the plugin's own default (it changed in 1.2.0) — set the value so
       it matches the migration-wide `backendRefName`.
     - `<labels>` set to the `routeLabels` input, in Maven plugin label format:
       `<labels><label><key>my/special-key</key><value>value1</value></label></labels>`.
       Do not invent values.
     - Do **not** set `HTTPROUTES_GENERATOR_GATEWAYS` in `values.yaml` — the plugin's default
       (`public-gateway`, `private-gateway`, `internal-gateway-service`) is the set this
       migration targets; only set it if the service genuinely needs a different/additional
       gateway list.
     - **Multi-module reactor:** bind the plugin execution in the last module of the reactor or a
       dedicated aggregator module — it runs in aggregator mode and needs every scanned module
       already compiled.
  2. **Confirm `<outputFile>`** is set to a path inside the Helm chart templates
     directory. This file must be committed to the branch.
  3. **Build the project** whenever Step 1 or this step changed `pom.xml`, including a
     `routes-registrator` bump made while the plugin was already present. A compatible plugin
     does not skip this build. Run `mvn -q clean process-classes` if Maven is available and
     record the command and exit code under `commandsRun:` and `done:`. If neither Maven step
     changed configuration, record that the build was not required. If Maven is not available,
     record under `skipped:` ("mvn not available in environment") and continue.
     If Maven is available but exits non-zero, map the plugin's failure message to
     `needsReview:` verbatim before failing — these are expected, by-design build failures, not
     tooling errors:
     - an unresolved `${...}` property placeholder in a route path;
     - a route that cannot be expressed as a `PathPrefix` + `ReplacePrefixMatch` rewrite (tail
       mismatch between the "from" and "to" paths);
     - two roots reducing to the same match with different rewrite targets;
     - a root reducing to `/` (a class-level `@Route` / `@GatewayRequestMapping` with no path; a
       broad prefix can match additional requests as a fallback when no higher-precedence rule
       applies);
     - the framework guard (wrong plugin for the detected framework — see the README).

     Then → `status: failed` per the contract.
  4. **Leave the generated `<outputFile>` in the working tree — do not run `git add` or
     `git commit`.** Remind the user:
     > The generated file must be committed. Every time route annotations change, run
     > `mvn clean process-classes` locally and commit the updated output file before raising a PR.
  5. Record the selected plugin artifact, version, and generated file path under `done:`.

The plugin outputs a gateway-group route
(`{{ .Values.SERVICE_NAME }}-java-annotations`, parented to the three border gateways by default)
and, only if any facade route needs a rewrite or a longer-than-default timeout, a facade route
(`{{ .Values.SERVICE_NAME }}-java-annotations-facade`) — split into multiple CRs if a route
exceeds the Gateway API 16-rule limit. It never generates a per-type
`-public`/`-private`/`-internal` HTTPRoute and never generates an `AuthorizationPolicy` — see
[`public-gateway-authz-policy`](../public-gateway-authz-policy/SKILL.md) for the recommended (manual by
default) workflow for public-gateway access control.

---

## Output

Write the contract report file (see [Contract → Outputs](#outputs)), then print
a short chat summary: files edited, commands run with exit codes, skipped items,
and every `needsReview:` entry.

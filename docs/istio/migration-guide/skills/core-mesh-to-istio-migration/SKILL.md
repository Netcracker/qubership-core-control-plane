---
name: core-mesh-to-istio-migration
description: >
  Orchestrates the full Cloud-Core Mesh to Istio migration described in
  docs/istio/migration-guide/core-mesh-to-istio-migration-guide.md. Converts
  declarative mesh CRs to Gateway API resources, migrates route-registration
  libraries, wires SERVICE_MESH_TYPE, adds the Java HTTPRoute generator when
  needed, generates HTTPRoutes from Go/Java route registration code, validates
  Istio guards, and maintains MIGRATION_LOG.md. Use when the user asks to run a
  full Istio migration, migrate Core Mesh to Istio, migrate to Istio Ambient
  Mesh, or execute the migration guide end-to-end.
---

# Core Mesh → Istio — Full Migration Orchestrator

This skill runs the complete migration described in the guide.
It is an **orchestrator**: the heavy lifting lives in two atomic sub-skills, and this
skill coordinates them, performs the remaining steps, and keeps an auditable log.

## Sub-skills invoked

| Sub-skill                                                                           | Used in step | Purpose                                                        |
| ----------------------------------------------------------------------------------- | ------------ | -------------------------------------------------------------- |
| [`qubership-mesh-to-istio`](../qubership-mesh-to-istio/SKILL.md)                    | Step 1       | Convert existing Helm mesh CRs to Gateway + HTTPRoute          |
| [`httproute-from-code`](../httproute-from-code/SKILL.md)                            | Step 2.4     | Generate HTTPRoute CRs from Go/Java route registration code    |

When a step calls for work already covered by a sub-skill, **invoke the sub-skill
exactly as written in its `SKILL.md`** — do not re-implement its logic here.

---

## Inputs the agent must resolve up front

Before starting any step, confirm or ask the user for:

1. **Chart path** — path to the Helm chart to migrate (e.g. `helm-templates/my-service`).
2. **Source code path** — path to Go/Java route registration code (often `./` or `src/`).
3. **Service language(s)** — Go, Java, or both. Affects Step 2 substeps.
4. **Build system** — Maven (Java) or `go.mod` (Go). Needed for Step 2.

If any is missing, ask before proceeding. Do not guess the chart path.

---

## Migration log — MANDATORY

The skill **must** create and continuously update a migration log at the repo root:

```
MIGRATION_LOG.md
```

The log is the single source of truth for what the automation did. It is updated
**after every step** — never wait until the end.

### Log structure

````markdown
# Core Mesh → Istio Migration Log

Started: <ISO-8601 timestamp>
Chart:   <chart path>
Code:    <code path>
Language: <Go | Java | Go+Java>

---

## Done
<!-- Items fully applied by automation. One bullet per concrete change. -->

## Skipped
<!-- Items intentionally not applied, with reason. -->

## Needs review
<!-- Items the user MUST verify before merging. Each entry MUST include:
     - File / location
     - Why it needs human review
     - Suggested action -->

## Per-step status

| Step | Title                                       | Status      | Notes |
|------|---------------------------------------------|-------------|-------|
| 1    | Migrate mesh CRs → HTTPRoute CRs            | pending     |       |
| 1.1  | Log manually handle flagged features        | pending     |       |
| 2.1  | Switch to mesh-aware route libraries        | pending     |       |
| 2.2  | Set SERVICE_MESH_TYPE env var               | pending     |       |
| 2.3  | Add Maven plugin (Java only)                | pending     |       |
| 2.4  | Generate HTTPRoute CRs from code            | pending     |       |
| 2.5  | Verify HTTPRoutes are Istio-guarded         | pending     |       |
````

### Logging rules

- **Do:** append concrete file paths, resource names, counts, and commands you ran.
- **Do:** classify every non-trivial action as **Done**, **Skipped**, or **Needs review**.
- **Do:** echo a short chat summary of the log update after each step
  (`Updated MIGRATION_LOG.md — 3 done, 1 needs review`).
- **Don't:** overwrite the log — always append.
- **Don't:** delete a `Needs review` entry until the user confirms it is resolved.

### What belongs in each bucket

| Bucket           | Examples                                                                                                                                                                                |
| ---------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Done**         | Files wrapped in Core/Istio guards, generated `-istio.yaml` files, HTTPRoutes emitted from code, Maven plugin added, env var wired, library versions bumped, values.yaml / schema updated |
| **Skipped**      | Maven plugin for a Go-only service, library swap for a language that is not present, a step the user explicitly said to defer                                                           |
| **Needs review** | `RouteConfiguration.spec.tlsSupported` / `.overridden` non-empty, `VirtualService.rateLimit` / `.overridden` non-empty, `*` host on an east-west route, `RouteDestination.cluster` / `.tlsSupported` / `.tlsEndpoint` / `.tlsConfigName` / `.httpVersion` / `.circuitBreaker` / `.tcpKeepalive` non-empty, `Rule.rateLimit` / `.luaFilter` non-empty, `Rule.allowed` / `.deny` / `.idleTimeout` / `.statefulSession` non-nil (whole `StatefulSession` block), `FacadeService` with no port defined, named `{{- include }}` helpers producing mesh CRs, unresolved gateway references, missing microservice name, any ambiguity reported by a sub-skill, unknown library versions |

---

## Execution plan (Step 1 + Step 2 substeps)

Run steps in order. After each step: update the log and print a one-line status.

### Step 1 — Migrate existing mesh CRs to Gateway API CRs

1. Invoke the sub-skill [`qubership-mesh-to-istio`](../qubership-mesh-to-istio/SKILL.md)
   with the chart path.
2. That skill will: wrap originals in `SERVICE_MESH_TYPE=Core` guards, generate
   `-istio.yaml` siblings guarded by `SERVICE_MESH_TYPE=Istio`, convert
   `Gateway(ingress/egress)` → Istio Gateway, convert `RouteConfiguration`
   → HTTPRoute, omit `FacadeService` and mesh-type `Gateway` (generates east-west HTTProutes instead, where parent is of kind Service, processed by waypoint proxy),
   and update `values.yaml` / `values.schema.json`.
3. **If the sub-skill pauses to ask about unresolved gateways** → forward the
   question to the user verbatim, wait for the answer, and resume the sub-skill.
   Log each decision under **Needs review** → move to **Done** once applied.
4. Copy the sub-skill's output summary (modified / generated files, transformed
   resource counts, manual-review list) into the log.

Log update:
- **Done:** every file in `Files modified` and `Files generated`.
- **Needs review:** every item from the sub-skill's "Items needing manual review".

After this step, verify:

```bash
helm template <chart> --set SERVICE_MESH_TYPE=Istio | grep -E 'kind: (HTTPRoute|Gateway)'
```

The command must succeed and return matching lines. Log the command and its
exit code under **Done** (or **Needs review** if it fails).

### Step 1.1 — Log manually handle flagged features

For each `# ⚠ MANUAL REVIEW REQUIRED` comment the sub-skill emitted, classify and log it. **None of these are safe to auto-fix** — they all require human judgement or a design change. Leave the flag in place and add a **Needs review** entry.

### Step 2 — Migrate non-declarative routes

Run the following substeps for services that use route-registration libraries or
route-registration annotations. Keep all generated HTTPRoute files committed in
the branch and remind the user to rerun the generation whenever route annotations
or registration code change.

### Step 2.1 — Switch to mesh-type-aware route-registration libraries

Apply only to languages actually present in the repo.

#### Java

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
- If dependency management or a BOM already pins a compliant version, reuse it
  and log under **Done**. If the current artifact is ambiguous, add a
  **Needs review** entry instead of guessing between the Spring webclient and
  resttemplate variants.

#### Go

- In `go.mod`, find `github.com/netcracker/qubership-core-lib-go-rest-utils/v2`.
- If present with version `>= v2.5.0` → log under **Done**.
- If present with a lower version → bump to at least `v2.5.0`, run
  `go mod tidy`, and log the exit code.
- If absent → do not add it automatically; add a **Needs review** entry
  ("Go route-registration dependency not found — confirm the service does not
  register routes in code").

Log update:
- **Done:** each dependency already compliant or updated; `go mod tidy` exit code.
- **Skipped:** language/framework not present.
- **Needs review:** ambiguous Java route-registration artifact choice, unknown versions, or missing route-registration dependencies.

### Step 2.2 — Set the `SERVICE_MESH_TYPE` environment variable

All services that use route registration libraries must receive
`SERVICE_MESH_TYPE`. By default, set Helm values to `Core` for compatibility with
environments where Istio is not installed yet; deployments can override the value
to `Istio` when migrating an environment.

| Deployment source                          | Action                                                                                           |
| ------------------------------------------ | ------------------------------------------------------------------------------------------------ |
| Helm values drive a `Deployment` template  | Ensure `values.yaml` has `SERVICE_MESH_TYPE: "Core"` and the Deployment `env:` uses `value: '{{ .Values.SERVICE_MESH_TYPE }}'`. |
| `values.schema.json` exists                | Ensure `SERVICE_MESH_TYPE` is a string enum with `Core` and `Istio`, default `Core`, and an explanatory description. |
| Plain Kubernetes `Deployment` manifest     | Add `- name: SERVICE_MESH_TYPE` with `value: Core`, or template it if the manifest is Helm-rendered. |

Log under **Done** the exact files edited. If multiple Deployments exist, list
each. If the desired runtime mesh for an environment is unclear, keep the default
`Core` and add a **Needs review** entry telling the user where to set `Istio`.

### Step 2.3 — Add the Maven plugin (Java services only)

- **If no `pom.xml`** → skip this step. Log under **Skipped**
  ("No pom.xml found — Go-only service").
- **If the Java service does not use route-registration annotations** → skip this
  step and log the reason.
- **If `pom.xml` exists and annotations are used**:
  1. Read the current `pom.xml`.
  2. If `httproutes-generator-maven-plugin` is already present → log under
     **Done** ("already present") and continue.
  3. Otherwise insert the plugin block from the migration guide with:
     - `<groupId>org.qubership.cloud.core</groupId>`
     - `<artifactId>httproutes-generator-maven-plugin</artifactId>`
     - `<version>0.0.1-SNAPSHOT</version>`
     - `<goal>generate-routes</goal>`
     - `<packages>` resolved from `src/main/java/...`. If ambiguous, set
       `com.example` and add a **Needs review** entry.
     - `<servicePort>` read from `application.yaml` / `application.properties`
       (`server.port`) or default to `8080` and add a **Needs review** entry.
     - `<outputFile>` pointing inside the chart templates directory, defaulting
       to `<chart>/templates/annotations-httproutes.yaml`.
     - `<backendRefVal>{{ .Values.DEPLOYMENT_RESOURCE_NAME }}</backendRefVal>`.
  4. Run `mvn clean compile` if available in the environment. Log exit code.
     If it fails, add a **Needs review** entry with the error summary.

Log update:
- **Done:** `pom.xml` edited or plugin already present; `mvn clean compile` exit code.
- **Skipped:** non-Java service or no route-registration annotations.
- **Needs review:** any default value that could not be confirmed from project files.

### Step 2.4 — Generate HTTPRoute CRs from route registration code

1. Invoke sub-skill [`httproute-from-code`](../httproute-from-code/SKILL.md) with
   the source-code path.
2. That skill scans Go (`*.go`) and Java (`*.java`) files, extracts
   `routeregistration.Route` / `RouteEntry` definitions, groups by `RouteType`,
   and emits one HTTPRoute CR per type to
   `helm-templates/<service name>/templates/source-code-httproutes.yaml`.
3. Copy the sub-skill's summary table into the log.
4. For every row where `Skipped = yes` or the skill emitted an `ERROR:` section,
   add a **Needs review** entry.
5. If the sub-skill fell back to `<microservice-name>` as the service name, add a
   **Needs review** entry ("Microservice name could not be resolved").

### Step 2.5 — Verify all HTTPRoutes are wrapped in Istio conditionals

1. List every file under `<chart>/templates/` (and any generated
   `-istio.yaml` / `source-code-httproutes.yaml`) that contains
   `kind: HTTPRoute`.
2. For each file, confirm the HTTPRoute block is inside a single
   `{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}` … `{{- end }}`. If a file
   has multiple HTTPRoute documents, the guard must wrap the whole block with
   `---` separators kept inside.
3. If a file is missing the guard → add it (safe automatic fix). Log under
   **Done**.
4. Run the negative-test dry-run:

   ```bash
   helm template <chart> --set SERVICE_MESH_TYPE=Core | grep 'kind: HTTPRoute'
   ```

   Expected output: empty. Record the command and exit status.
   - Empty → log under **Done**.
   - Any HTTPRoute leaks → log under **Needs review** with the offending file path.

---

## Final checklist and hand-off

Before declaring the migration complete, produce a **Final report** that mirrors
the "Final Checklist" in the migration guide, with a check mark only for boxes
that have at least one corresponding **Done** entry and no unresolved **Needs
review** entry:

```markdown
## Final report

- [x/ ] Existing mesh CRs converted to HTTPRoute CRs
- [x/ ] Flagged features from Step 1.1 resolved
- [x/ ] Mesh-aware libraries replace old route-posting libraries
- [x/ ] SERVICE_MESH_TYPE set in Helm values / Deployment
- [x/ ] Maven plugin added and local build passes (Java only)
- [x/ ] HTTPRoute CRs generated from route registration code
- [x/ ] All HTTPRoute CRs wrapped in the Istio conditional

Open items (require user review):
- <list all remaining "Needs review" entries from MIGRATION_LOG.md>
```

Close with a plain-language summary telling the user:

1. **What was applied automatically** (reference the Done section count).
2. **What was skipped and why** (reference the Skipped section).
3. **What requires careful human review before merging** (enumerate the Needs
   review section, highlighting items that could change runtime behavior —
   `RouteConfiguration.spec.tlsSupported` / `.overridden`, `rateLimit`,
   `VirtualService.overridden`, `*` hosts on east-west routes,
   `RouteDestination` TLS / `cluster` / `httpVersion` / `circuitBreaker` /
   `tcpKeepalive`, `Rule.allowed` / `.deny`, `Rule.statefulSession`,
   `Rule.idleTimeout`, `Rule.luaFilter`, `FacadeService` with no port,
   unresolved gateways, helper-produced CRs, placeholder library versions).
4. The recommended validation commands the user should run locally before
   pushing:

   ```bash
   helm template <chart> --set SERVICE_MESH_TYPE=Istio | grep -E 'kind: (HTTPRoute|Gateway)'
   helm template <chart> --set SERVICE_MESH_TYPE=Core  | grep 'kind: HTTPRoute'  # expect empty
   ```

---

## Operating rules

- **Never skip the log.** If the log file cannot be written, stop and report.
- **Never invent values.** Versions, package names, ports, microservice names —
  if unknown, add a **Needs review** entry instead of guessing.
- **Never bypass a sub-skill's user prompt.** If `qubership-mesh-to-istio` asks
  about an unresolved gateway, forward the question before proceeding.
- **Never run destructive commands.** Do not push, tag, or delete branches. Do
  not modify git config.
- **Be explicit in chat.** After each step, print a one-line summary plus the
  updated per-step status row.
- **Idempotent reruns.** Before editing a file, check whether the change is
  already in place; if yes, log under **Done** ("already present") and continue.

---

## Non-goals

- Do not generate `VirtualService`, `Ingress`, `GRPCRoute`, or `TCPRoute` — only
  Gateway API `Gateway` and `HTTPRoute`.
- Do not refactor application logic. The skill only touches:
  Helm templates, `values.yaml`, `values.schema.json`, `pom.xml`, `go.mod`,
  and `MIGRATION_LOG.md`.
- Do not raise pull requests automatically. Stop at a clean working tree for
  the user to review and commit.

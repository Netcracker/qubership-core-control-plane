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

**How to invoke a sub-skill:** read its `SKILL.md` in full and execute its steps
inline as an embedded procedure. Do not spawn subprocesses or defer to external
tooling — carry out each instruction as if it were written directly in this skill.

---

## Inputs the agent must resolve up front

Before starting any step, confirm or ask the user for:

1. **Chart path** — path to the Helm chart to migrate (e.g. `helm-templates/my-service`).
2. **Source code path** — path to Go/Java route registration code (often `./` or `src/`).
3. **Service language(s)** — Go, Java, or both. Affects Step 2 substeps.
4. **Build system** — Maven (Java) or `go.mod` (Go). Needed for Step 2.

If any is missing, ask before proceeding. Do not guess the chart path.

---

## Error policy — read before executing any step

If a step or sub-skill reports an unrecoverable error (non-zero exit code from a
required build command, a sub-skill `ERROR:` section, a file that cannot be
written), apply the following procedure **immediately**:

1. Stop the current step. Do not proceed to the next step.
2. Log the error under **Needs review** with the full error summary, the file or
   command that failed, and a suggested remediation.
3. Print a chat message:
   > ⛔ Step `<N>` failed: `<one-line error>`. Logged under Needs review.
   > Reply **continue** to skip this step and proceed, or **abort** to stop the migration.
4. Wait for the user's reply before taking any further action.

Optional steps (e.g. `mvn -q clean compile` when Maven is not in the environment)
may be skipped without user confirmation; log them under **Skipped** with the
reason.

---

## Migration log — MANDATORY

The skill **must** create and continuously update a migration log at the repo root:

```
MIGRATION_LOG.md
```

The log is the single source of truth for what the automation did. It is updated
**after every step** — never wait until the end. If the log file cannot be
written for any reason, stop immediately and report the failure to the user.

### Log structure

````markdown
# Core Mesh → Istio Migration Log

Started: <ISO-8601 timestamp>
Chart:   <chart path>
Code:    <code path>
Language: <Go | Java | Go+Java>

---

## Done
**Items fully applied by automation. One bullet per concrete change.**

## Skipped
**Items intentionally not applied, with reason.**

## Needs review
**Items the user MUST verify before merging. Each entry MUST include:**
**- File / location**
**- Why it needs human review**
**- Suggested action**

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

## Commands run

| Step | Command | Exit code | Notes |
|------|---------|-----------|-------|
````

> **Note:** The log uses bold text (not HTML comments) for section descriptions
> so they are preserved across all Markdown renderers and are re-parseable by
> the agent on idempotent reruns.

### Logging rules

- **Do:** append concrete file paths, resource names, counts, and commands you ran.
- **Do:** classify every non-trivial action as **Done**, **Skipped**, or **Needs review**.
- **Do:** record every command and its exit code in the **Commands run** table.
- **Do:** echo a short chat summary of the log update after each step
  (`Updated MIGRATION_LOG.md — 3 done, 1 needs review`).
- **Don't:** overwrite the log — always append.
- **Don't:** delete a `Needs review` entry until the user confirms it is resolved.

### What belongs in each bucket

**Structural blockers** — fields or patterns that cannot be auto-converted and
block a correct migration:

| Item | Example location |
|------|-----------------|
| `RouteConfiguration.spec.tlsSupported` / `.overridden` non-empty | RouteConfiguration CR |
| `VirtualService.rateLimit` / `.overridden` non-empty | VirtualService CR |
| `RouteDestination.cluster` / `.tlsSupported` / `.tlsEndpoint` / `.tlsConfigName` / `.httpVersion` / `.circuitBreaker` / `.tcpKeepalive` non-empty | RouteConfiguration CR |
| `Rule.rateLimit` / `.luaFilter` non-empty | RouteConfiguration CR |
| `Rule.allowed` / `.deny` / `.idleTimeout` / `.statefulSession` non-nil | RouteConfiguration CR |
| `FacadeService` with no port defined | FacadeService CR |
| Named `{{- include }}` helpers producing mesh CRs | Helm templates |
| `*` host on an east-west route | Generated HTTPRoute |

**Unknown values** — values the agent cannot safely infer and must not guess:

| Item | Example location |
|------|-----------------|
| Unresolved gateway references | HTTPRoute `parentRefs` |
| Missing microservice name (placeholder `<microservice-name>` in output) | Generated HTTPRoute / `source-code-httproutes.yaml` |
| Ambiguous Java route-registration artifact (webclient vs resttemplate) | `pom.xml` |
| Unknown library versions | `pom.xml` / `go.mod` |

**Done** examples: files wrapped in Core/Istio guards, generated `-istio.yaml`
files, HTTPRoutes emitted from code, Maven plugin added, env var wired, library
versions bumped, `values.yaml` / `values.schema.json` updated, commands that
exited 0.

**Skipped** examples: Maven plugin for a Go-only service, library swap for a
language not present, a step the user explicitly said to defer, optional build
commands not available in the environment.

---

## Execution plan (Step 1 + Step 2 substeps)

Run steps in order. After each step: update the log, record the per-step status
row, and print a one-line chat status.

### Step 1 — Migrate existing mesh CRs to Gateway API CRs

**Idempotency check:** before running, scan `<chart>/templates/` for files
already containing `kind: HTTPRoute` guarded by
`{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}`. If the full output of this
step already exists, log all affected files under **Done** ("already present")
and skip to Step 1.1.

1. Invoke the sub-skill [`qubership-mesh-to-istio`](../qubership-mesh-to-istio/SKILL.md)
   with the chart path.
2. That skill will: wrap originals in `SERVICE_MESH_TYPE=Core` guards, generate
   `-istio.yaml` siblings guarded by `SERVICE_MESH_TYPE=Istio`, convert
   `Gateway(ingress/egress)` → Istio Gateway, convert `RouteConfiguration`
   → HTTPRoute, omit `FacadeService` and mesh-type `Gateway` (generates
   east-west HTTPRoutes instead, where parent is of kind Service, processed by
   waypoint proxy), and update `values.yaml` / `values.schema.json`.
3. **If the sub-skill pauses to ask about unresolved gateways** → forward the
   question to the user verbatim, wait for the answer, and resume the sub-skill.
   Log each decision under **Needs review** → move to **Done** once applied.
4. Copy the sub-skill's output summary (modified / generated files, transformed
   resource counts, manual-review list) into the log.

Log update:
- **Done:** every file in `Files modified` and `Files generated`.
- **Needs review:** every item from the sub-skill's "Items needing manual review".

**Validation:**

```bash
helm dependency update && helm template <chart> --set SERVICE_MESH_TYPE=Istio \
  | grep -E 'kind: (HTTPRoute|Gateway)'
```

Expected: command exits 0 and returns at least one matching line. Record the
command and exit code in the **Commands run** table. If it fails, apply the
[Error policy](#error-policy--read-before-executing-any-step).

### Step 1.1 — Log manually handle flagged features

For each `# ⚠ MANUAL REVIEW REQUIRED` comment the sub-skill emitted, add a
**Needs review** entry. **None of these are safe to auto-fix** — they all
require human judgement or a design change. Leave the flag comment in place in
the file; do not remove it.

### Step 2 — Migrate non-declarative routes

Run the following substeps for services that use route-registration libraries or
route-registration annotations. Keep all generated HTTPRoute files committed in
the branch and remind the user to rerun the generation whenever route annotations
or registration code change.

### Step 2.1 — Switch to mesh-type-aware route-registration libraries

Apply only to languages actually present in the repo.

#### Java

**Idempotency check:** for each dependency below, check whether the current
version already satisfies the minimum. If yes, log under **Done** ("already
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
- If the choice between webclient and resttemplate variants is ambiguous, add a
  **Needs review** entry (unknown artifact choice) rather than guessing.

#### Go

**Idempotency check:** read `go.mod` before making any changes.

- In `go.mod`, find `github.com/netcracker/qubership-core-lib-go-rest-utils/v2`.
- If present with version `>= v2.5.0` → log under **Done** ("already compliant"). No changes needed.
- If present with a lower version → bump to at least `v2.5.0`, run
  `go mod tidy`, and record the exit code in **Commands run**. If `go mod tidy`
  exits non-zero, apply the [Error policy](#error-policy--read-before-executing-any-step).
- If absent → do not add it automatically; add a **Needs review** entry:
  "Go route-registration dependency not found — confirm the service does not
  register routes in code."
- If the repo contains a `go.work` file (Go workspace), add a **Needs review**
  entry: "Go workspace (`go.work`) detected — multi-module dependency bumps are
  out of scope for this skill and require manual handling."
- If multiple modules import `rest-utils` at different versions, add a
  **Needs review** entry for each conflicting module.

Log update:
- **Done:** each dependency already compliant or updated; `go mod tidy` exit code.
- **Skipped:** language/framework not present.
- **Needs review:** ambiguous artifact choice, unknown versions, missing
  route-registration dependency, Go workspace, or conflicting module versions.

### Step 2.2 — Set the `SERVICE_MESH_TYPE` environment variable

**Idempotency check:** before editing any file, check whether `SERVICE_MESH_TYPE`
is already present with the correct value and schema entry. If yes, log under
**Done** ("already present") for that file and skip it.

All services that use route registration libraries must receive
`SERVICE_MESH_TYPE`. By default, set Helm values to `Core` for compatibility with
environments where Istio is not installed yet; deployments can override the value
to `Istio` when migrating an environment.

| Deployment source                          | Action                                                                                           |
| ------------------------------------------ | ------------------------------------------------------------------------------------------------ |
| Helm values drive a `Deployment` template  | Ensure `values.yaml` has `SERVICE_MESH_TYPE: "Core"` and the Deployment `env:` uses `value: '{{ .Values.SERVICE_MESH_TYPE }}'`. |
| `values.schema.json` exists                | Ensure `SERVICE_MESH_TYPE` has a full schema entry: `"type": "string"`, `"enum": ["Istio", "Core"]`, `"default": "Core"`, `"$id": "#/properties/SERVICE_MESH_TYPE"`, `"internal": true`, exact `"description": "Service mesh type. Use `Core` for Cloud Core Mesh or `Istio` for Istio Ambient Mesh."`, and an entry in the root-level `"examples"` array (`{"SERVICE_MESH_TYPE": "Core"}`). Also confirm `"additionalProperties": true` is set at the root. |
| Plain Kubernetes `Deployment` manifest     | Add `- name: SERVICE_MESH_TYPE` with `value: Core`, or template it if the manifest is Helm-rendered. |

Log under **Done** the exact files edited. If multiple Deployments exist, list
each. If the desired runtime mesh for an environment is unclear, keep the default
`Core` and add a **Needs review** entry telling the user where to set `Istio`.

### Step 2.3 — Add the Maven plugin (Java services only)

**Idempotency check:** if `httproutes-generator-maven-plugin` is already present
in `pom.xml`, log under **Done** ("already present") and skip to Step 2.4.

- **If no `pom.xml`** → skip this step. Log under **Skipped**
  ("No pom.xml found — Go-only service").
- **If the Java service does not use route-registration annotations** → skip this
  step and log the reason.
- **If `pom.xml` exists and annotations are used**, follow these five sub-steps
  (from the [plugin README](https://github.com/Netcracker/qubership-core-control-plane/blob/main/httproute-generator/README.md)):
  1. **Add plugin to `pom.xml`** with the following configuration:
     - `<groupId>org.qubership.cloud.core</groupId>`
     - `<artifactId>httproutes-generator-maven-plugin</artifactId>`
     - `<version>0.0.1-SNAPSHOT</version>`
     - `<phase>generate-resources</phase>`
     - `<goal>generate-routes</goal>`
     - `<packages>` resolved from `src/main/java/...`. If ambiguous, set
       `com.example` and add a **Needs review** entry.
     - `<servicePort>` read from `application.yaml` / `application.properties`
       (`server.port`) or default to `8080` and add a **Needs review** entry.
     - `<outputFile>` pointing inside the chart templates directory, defaulting
       to `<chart>/templates/annotations-httproutes.yaml`.
     - `<backendRefVal>{{ .Values.DEPLOYMENT_RESOURCE_NAME }}</backendRefVal>`.
  2. **Confirm `<outputFile>`** is set to a path inside the Helm chart templates
     directory (see above). This file must be committed to the branch.
  3. **Build the project** to generate the output file. Run `mvn -q clean compile`
     if Maven is available in the environment. Record the exit code in
     **Commands run**. If Maven is not available, log under **Skipped**
     ("mvn not available in environment") and continue. If Maven is available
     but exits non-zero, apply the
     [Error policy](#error-policy--read-before-executing-any-step).
  4. **Commit the generated `<outputFile>`** to the branch. Remind the user:
     > The plugin generates the output file at compile time. Every time route
     > annotations change, run `mvn clean compile` locally and commit the updated
     > output file before raising a PR.
  5. Log the committed file path under **Done**.

Log update:
- **Done:** `pom.xml` edited; `mvn -q clean compile` exit code (if run); generated
  output file path committed.
- **Skipped:** non-Java service, no route-registration annotations, or Maven
  not available in the environment.
- **Needs review:** any default value that could not be confirmed from project
  files (`<packages>`, `<servicePort>`).

### Step 2.4 — Generate HTTPRoute CRs from route registration code

**Idempotency check:** if `source-code-httproutes.yaml` already exists and its
content matches what the sub-skill would generate (no source-code changes since
last run), log under **Done** ("already present") and skip.

1. Invoke sub-skill [`httproute-from-code`](../httproute-from-code/SKILL.md) with
   the source-code path.
2. That skill scans Go (`*.go`) and Java (`*.java`) files, extracts
   `routeregistration.Route` / `RouteEntry` definitions, groups by `RouteType`,
   and emits one HTTPRoute CR per type to
   `helm-templates/<service name>/templates/source-code-httproutes.yaml`.
3. **If the sub-skill fell back to `<microservice-name>` as the service name:**
   - Do **not** leave the file with the literal placeholder in place — it will
     break `helm template` silently.
   - Rename the output file to `source-code-httproutes.yaml.incomplete` and add
     a prominent comment at the top: `# INCOMPLETE: replace <microservice-name>
     before committing`.
   - Add a **Needs review** entry: "Microservice name could not be resolved —
     file renamed to `.incomplete`; set the correct name and rename before
     merging."
4. **Commit all generated files** to the branch. Remind the user:
   > The generated `source-code-httproutes.yaml` must stay committed. Any time
   > route registration code changes, rerun the `httproute-from-code` skill and
   > commit the updated output before raising a PR.
5. Copy the sub-skill's summary table into the log.
6. For every row where `Skipped = yes` or the skill emitted an `ERROR:` section,
   add a **Needs review** entry.

### Step 2.5 — Verify all HTTPRoutes are wrapped in Istio conditionals

**Idempotency check:** if all HTTPRoute files already have the correct guard,
log under **Done** ("already guarded") for each and skip to validation.

1. List every file under `<chart>/templates/` (and any generated
   `-istio.yaml` / `source-code-httproutes.yaml`) that contains
   `kind: HTTPRoute`.
2. For each file, confirm the HTTPRoute block is inside a single
   `{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}` … `{{- end }}`. If a file
   has multiple HTTPRoute documents, the guard must wrap the whole block with
   `---` separators kept inside.
3. If a file is missing the guard → add it (safe automatic fix). Log under
   **Done**.

**Validation:**

```bash
helm dependency update && helm template <chart> --set SERVICE_MESH_TYPE=Core \
  | grep 'kind: HTTPRoute'
```

Expected output: empty (no HTTPRoutes leak under Core mode). Record the command
and exit code in **Commands run**.
- Empty output → log under **Done**.
- Any HTTPRoute lines in output → log each offending file path under
  **Needs review** and apply the
  [Error policy](#error-policy--read-before-executing-any-step).

---

## Final checklist and hand-off

Before declaring the migration complete, produce a **Final report** that mirrors
the "Final Checklist" in the migration guide. Mark `[x]` only when the step has
at least one **Done** entry and zero unresolved **Needs review** entries:

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
   review section, highlighting structural blockers that could change runtime
   behaviour — `RouteConfiguration.spec.tlsSupported` / `.overridden`,
   `rateLimit`, `VirtualService.overridden`, `*` hosts on east-west routes,
   `RouteDestination` TLS / `cluster` / `httpVersion` / `circuitBreaker` /
   `tcpKeepalive`, `Rule.allowed` / `.deny`, `Rule.statefulSession`,
   `Rule.idleTimeout`, `Rule.luaFilter`, `FacadeService` with no port,
   unresolved gateways, helper-produced CRs, placeholder library versions, and
   any `.incomplete` files from Step 2.4).
4. The recommended validation commands the user should run locally before pushing:

   ```bash
   # Must return at least one HTTPRoute or Gateway line
   helm template <chart> --set SERVICE_MESH_TYPE=Istio \
     | grep -E 'kind: (HTTPRoute|Gateway)'

   # Must return nothing — HTTPRoutes must not leak under Core mode
   helm template <chart> --set SERVICE_MESH_TYPE=Core \
     | grep 'kind: HTTPRoute'
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
- **Idempotent reruns.** Each step begins with an explicit idempotency check
  (described inline). If the step's outputs are already in place and correct,
  log them as **Done** ("already present") and move on without re-applying changes.
- **Follow the Error policy.** On any unrecoverable failure, stop the step, log
  it, and ask the user whether to continue or abort before taking further action.

---

## Non-goals

This skill only modifies:
Helm templates, `values.yaml`, `values.schema.json`, `pom.xml`, `go.mod`,
and `MIGRATION_LOG.md`.

It does not raise pull requests, push branches, rewrite application logic, or
modify git configuration.
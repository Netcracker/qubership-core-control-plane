---
name: core-mesh-to-istio-migration
description: >
  Orchestrate the full Cloud-Core Mesh to Istio migration end-to-end — convert
  declarative mesh CRs to Gateway API (routes with path variables cut to
  PathPrefix, forbidden routes to AuthorizationPolicy DENY), migrate
  route-registration libraries, wire SERVICE_MESH_TYPE, add the Java HTTPRoute
  generator, generate HTTPRoutes from Go/Java code, validate Istio guards /
  duplicate rules / DENY overlaps / imperative control-plane calls, and maintain
  .mesh-migration/MIGRATION_LOG.md. Use when asked to migrate a
  service from Core Mesh to Istio (Ambient Mesh) or run the migration guide end-to-end.
---

# Core Mesh → Istio — Full Migration Orchestrator

This skill runs the complete migration described in the guide.
It is an **orchestrator**: the heavy lifting lives in four atomic sub-skills, and this
skill coordinates them, resolves user questions, and keeps an auditable log.

## Invocation

Run this skill against the chart or service directory to migrate. Examples:

- `mesh-test-service-go`
- `helm-templates/my-service`
- `.`

The user may also name a **start step** ("start from Step 2.4", "rerun the
validation") to resume a migration instead of running from scratch. Default is
a full run from Step 1. See the report lifecycle below for how a resumed run
treats earlier steps' reports.

## Sub-skills invoked

| Sub-skill                                                                           | Used in step | Purpose                                                        |
| ----------------------------------------------------------------------------------- | ------------ | -------------------------------------------------------------- |
| [`core-mesh-crs-to-istio`](../core-mesh-crs-to-istio/SKILL.md)                      | Step 1       | Convert existing Helm mesh CRs to Gateway API + Istio resources |
| [`mesh-build-wiring`](../mesh-build-wiring/SKILL.md)                                | Steps 2.1–2.3 | Mesh-aware libraries, SERVICE_MESH_TYPE env, Maven plugin      |
| [`httproute-from-code`](../httproute-from-code/SKILL.md)                            | Step 2.4     | Generate HTTPRoute CRs from Go/Java route registration code    |
| [`istio-migration-validate`](../istio-migration-validate/SKILL.md)                  | Steps 2.5–2.7 | Istio guards, render checks, duplicate rules, DENY overlaps, imperative control-plane calls |

`core-mesh-crs-to-istio` and `httproute-from-code` both follow the shared
[`regex-routes-migration`](../regex-routes-migration/SKILL.md) procedure for
routes with `{variables}` and forbidden routes: every path is cut to a
`PathPrefix`, forbidden paths on the public / private gateways become
`AuthorizationPolicy` DENY rules, and routes whose behavior would change come
back as `route-conflict/*` questions. **No step generates a `VirtualService`.**

**How to invoke a sub-skill:** communicate only through the sub-skill's
`## Contract` — resolved inputs in, report file out. Always pass
`interactive: false`: sub-skills must never address the user directly; their
questions come back as `unresolved:` report entries. When the harness provides
a sub-agent / Task tool, run the sub-skill in a sub-agent with its own context
and consume its report file afterwards. Otherwise read its `SKILL.md` in full
and execute its steps inline as an embedded procedure. Either way, take results
from the report file, not from the sub-skill's transcript.

## Sub-skill reports

Sub-skills write machine-readable reports to `.mesh-migration/reports/<skill>.yaml`.
Lifecycle:

- **Ensure `.mesh-migration/` is gitignored before the first executed step** —
  check the target repo's `.gitignore` for a `.mesh-migration/` entry and
  append it (with a short comment) if missing. Reports are working files and
  must never be committed. Log the edit under **Done**.
- **Full run (no start step): delete `.mesh-migration/reports/` and any
  existing `.mesh-migration/MIGRATION_LOG.md` before Step 1** — a fresh run
  must never read a previous run's answers or append to its log.
- **Resumed run (start step given): keep existing reports.** Steps before the
  start step are not re-executed — their results come from the reports on disk.
  Before executing anything, list every report that will be reused (`skill`,
  `generatedAt`, `status`) and tell the user: "Resuming from <step>; reusing
  these previous results — rerun from Step 1 instead if they are stale." Then:
  - a report **required by a later step is missing or `status: failed`**
    (e.g. Step 2.3 / 2.4 need `backendRef` and `labels` from the Step 1
    report) → ask the user to either start from the earlier step or provide
    the missing values as inputs; do not proceed silently.
  - a reused report has `status: partial` → treat its `unresolved:` entries
    exactly as if the sub-skill had just run (batch questions, deliver
    `resolutions`).
  - mark the skipped steps' rows in the per-step status table as
    `reused (report of <generatedAt>)`.
- **Never skip a sub-skill invocation to save work.** Idempotency belongs to
  the sub-skills, which detect their own already-present output; skipping at
  the orchestrator level leaves no report on disk, and every downstream step
  reads reports. The only steps that legitimately run without invoking are the
  ones before a resumed run's start step, which reuse the reports already
  there.
- After each sub-skill finishes, read its report. Ignore unknown fields. If
  `reportSchema` is missing or newer than the value documented in that skill's
  Contract, stop and add a **Needs review** entry (contract mismatch) instead of
  guessing field meanings.
- `status: partial` means the `unresolved:` list blocks part of the output.
  Relay each entry's `question` (and `options`, marking `default` as the
  recommended one) to the user verbatim, in **one** batched round — the question
  texts carry their own explanation (for example the BWC warning of
  `java-forbidden-routes`), so never shorten them. Deliver the answers as a `resolutions` map keyed by the
  entries' `id` — by **continuing the same sub-agent** when the harness
  supports it (context intact, no rework), otherwise by **re-invoking** the
  sub-skill with `resolutions` as an input (its idempotency checks make the
  second pass cheap).
- **After delivering `resolutions`, re-read the report and confirm
  `status: complete` before continuing to the next step.** If it is still
  `partial`, the answers did not resolve everything: repeat the question round
  for the remaining `unresolved:` entries. Do this at most twice; if the third
  read is still `partial`, stop, log each surviving entry under **Needs
  review**, and ask the user whether to continue with the incomplete output or
  abort. Never treat a `partial` report as done.
- **Exception — `blockedBy`:** a report with `status: partial`, an empty
  `unresolved:` and `blockedBy: java-forbidden-routes` (from
  `mesh-build-wiring`) means the user chose to refactor controllers. Do not
  re-ask; apply [Stop for a controller refactor](#stop-for-a-controller-refactor).

---

## Inputs the agent must resolve up front

Before starting any step, confirm or ask the user for:

1. **Chart path** — path to the Helm chart to migrate (e.g. `helm-templates/my-service`).
2. **Source code path** — path to Go/Java route registration code (often `./` or `src/`).
3. **Service language(s)** — Go, Java, or both. Affects Step 2 substeps.
4. **Build system** — Maven (Java) or `go.mod` (Go). Needed for Step 2.
5. **Start step** — optional; defaults to Step 1 (full run). When given, earlier
   steps are skipped and their reports reused (see the report lifecycle).

If any is missing, ask before proceeding. Do not guess the chart path.

### Service root (`repoRoot`) — do NOT ask up front

Pass `repoRoot` to [`istio-migration-validate`](../istio-migration-validate/SKILL.md)
as the **service root** that owns the chart and its scripts — the directory that
contains (or is the parent of) both the chart path and the source-code path when
they share a tree. Prefer the source-code path when it is already the service
root (e.g. `.`). Never widen to a multi-service monorepo git root that includes
sibling services; that floods Step 2.7 with unrelated hits.

### Backend reference (`backendRefName` / `backendRefPort`) — do NOT ask up front

The `backendRefs[].name` and `backendRefs[].port` applied to generated HTTPRoutes
are **migration-wide** values, but they are **not** collected at orchestrator
start. Resolve them as follows:

1. **Step 1** invokes [`core-mesh-crs-to-istio`](../core-mesh-crs-to-istio/SKILL.md),
   which detects the service's own backend `name`/`port` from the existing mesh
   `RouteConfiguration` destinations (one migrated service contains only routes
   for itself) and reports them in its report file's `backendRef` field.
2. **If Step 1 resolved both values** → capture them, record them in
   `.mesh-migration/MIGRATION_LOG.md` (under **Done**), and reuse them in
   **Step 2.3** and **Step 2.4** without prompting.
3. **If Step 1 could not resolve them** (no declarative mesh CRs, ambiguous /
   conflicting destinations, or Step 1 was skipped) → ask the user **explicitly**
   at the point they are first needed (Step 2.3 for Java, Step 2.4 otherwise),
   proposing the defaults `{{ .Values.DEPLOYMENT_RESOURCE_NAME }}` and `8080`.
   Do not ask earlier than necessary.

Once resolved (detected or user-provided), the same values MUST be:
- passed to the Maven plugin config in **Step 2.3** (`<backendRefVal>` and
  `<servicePort>` respectively), and
- propagated to the [`httproute-from-code`](../httproute-from-code/SKILL.md)
  sub-skill in **Step 2.4** so the generated `backendRefs` match.

### Route labels (`routeLabels`) — prefer Step 1 detection

The `metadata.labels` map used by generated HTTPRoutes must be consistent between:

- declarative CR migration output from
  [`core-mesh-crs-to-istio`](../core-mesh-crs-to-istio/SKILL.md) **Step 1**,
- Maven-plugin-generated routes in **Step 2.3**, and
- [`httproute-from-code`](../httproute-from-code/SKILL.md) output in **Step 2.4**.

Resolve labels as follows:

1. Capture `labels.values` from Step 1's report file
   (`.mesh-migration/reports/core-mesh-crs-to-istio.yaml`) — the map itself,
   not the whole `labels` object.
2. If `labels.values` is non-null, treat that map as migration-wide `routeLabels`.
3. If `labels.unresolvedReason` is set, ask the user for a label map only when
   first needed (Step 2.3 for Java or Step 2.4 otherwise), and record it in the log.
4. Never silently invent label values.

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

Keep `.mesh-migration/MIGRATION_LOG.md` updated **after every step**; if it
cannot be written, stop and report the failure. **Before Step 1, read
[`migration-log.md`](migration-log.md) in full with the Read tool** — it defines
the log structure, the logging rules, and what belongs in **Done** /
**Skipped** / **Needs review**. Never overwrite the log — append.

---

## Execution plan (Step 1 + Step 2 substeps)

Run steps in order, beginning at the user-selected start step (Step 1 by
default). On a resumed run, apply the reused-report notification from the
[report lifecycle](#sub-skill-reports) before executing anything. After each
step: update the log, record the per-step status row, and print a one-line chat
status.

### Step 1 — Migrate existing mesh CRs to Gateway API CRs

**Idempotency:** always invoke the sub-skill — do not skip it because the chart
already contains `kind: HTTPRoute` guarded by
`{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}`. The sub-skill detects existing
output itself (its wrapping and generation steps are idempotent), reports
already-present files under **Done**, and — crucially — writes the current
report that Steps 2.3 and 2.4 read for `backendRef` and `labels.values`.

1. Invoke the sub-skill [`core-mesh-crs-to-istio`](../core-mesh-crs-to-istio/SKILL.md)
   with inputs: `chartPath: <chart path>`, `interactive: false`.
2. That skill will, in a single pass: wrap originals in `SERVICE_MESH_TYPE=Core`
   guards, generate `-istio.yaml` siblings guarded by `SERVICE_MESH_TYPE=Istio`,
   convert `Gateway(ingress/egress)` → Istio Gateway, convert
   `RouteConfiguration` → HTTPRoute (including any rule-level `statefulSession`
   → `DestinationRule`), convert egress `TlsDef` + `https://` destinations →
   `ServiceEntry` + cert `Secret` + TLS-origination `DestinationRule`, convert
   `FacadeService` → `Service`, omit mesh-type
   `Gateway` (generates east-west HTTPRoutes instead, where parent is of kind
   Service, processed by waypoint proxy), convert standalone `StatefulSession` /
   `LoadBalance` →
   `DestinationRule` (one per host — conflicting policies are flagged
   `⚠ MANUAL REVIEW` inside the skill), convert `HttpFilters` +
   `RouteConfiguration` Lua scripts → `TrafficExtension` (requires Istio
   ≥ 1.30), cut every route path with `{variables}` to a `PathPrefix`, turn
   `allowed: false` routes (and paths the cut newly exposes) on the public /
   private gateways into `AuthorizationPolicy` DENY rules, and update
   `values.yaml` / `values.schema.json`.
3. Read `.mesh-migration/reports/core-mesh-crs-to-istio.yaml`. **If
   `status: partial`**, collect every entry under `unresolved:` and ask the user
   all questions in **one batch** (for each unresolved gateway: ingress or
   mesh?; for each `route-conflict/*`: the question text verbatim with its
   options). Deliver the answers as a `resolutions` map keyed by entry `id` —
   continue the same sub-agent when possible, otherwise re-invoke the sub-skill
   with `resolutions` — and re-read the report. Log each decision under
   **Needs review** → move to **Done** once applied.
4. Copy the report's `filesModified` / `filesGenerated`, resource counts
   (including `authorizationPolicy`), and `needsReview` items into the log —
   the three AuthorizationPolicy precondition lines (Istio ≥ 1.22, path
   normalization, 404 → 403) go under **Needs review** once.
5. **Capture the detected backend reference** from the report's `backendRef`
   field. If both `name` and `port` are set, record them in the log (under
   **Done**) as the migration-wide backend reference to reuse in Step 2.3 /
   Step 2.4. If `unresolvedReason` is set, note that they must be asked from the
   user when first needed (see
   [Backend reference](#backend-reference-backendrefname--backendrefport--do-not-ask-up-front)).
6. **Capture the detected labels** from the report's `labels.values` map (not
   the enclosing `labels` object). If non-null, store that map as
   migration-wide `routeLabels` for Step 2.3 / Step 2.4. If
   `labels.unresolvedReason` is set, add a **Needs review** entry and ask
   the user only when labels are first needed.

Log update:
- **Done:** every file in `Files modified` and `Files generated`; the detected
  `backendRefName` / `backendRefPort` and `routeLabels` (if resolved).
- **Needs review:** every item from the sub-skill's "Items needing manual review".

### Step 1.1 — Log manually handle flagged features

For each `# ⚠ MANUAL REVIEW` comment the sub-skill emitted, add a
**Needs review** entry. **None of these are safe to auto-fix** — they all
require human judgement or a design change. Leave the flag comment in place in
the file; do not remove it.

### Step 2 — Migrate non-declarative routes

Run the following substeps for services that use route-registration libraries or
route-registration annotations. Keep all generated HTTPRoute files committed in
the branch and remind the user to rerun the generation whenever route annotations
or registration code change.

### Steps 2.1–2.3 — Wire build and deployment (delegate to `mesh-build-wiring`)

**Idempotency:** the sub-skill performs its own per-item idempotency checks and
records "already compliant / already present" items under `done:`.

1. Invoke the sub-skill [`mesh-build-wiring`](../mesh-build-wiring/SKILL.md)
   with: `codePath`, `chartPath`, `language`, `interactive: false`, and — for
   Java — the resolved
   `backendRefName` / `backendRefPort` / `routeLabels` (detected by Step 1, or
   asked from the user here if still unresolved — propose the defaults
   `{{ .Values.DEPLOYMENT_RESOURCE_NAME }}` / `8080`).
2. That skill will: switch route-registration libraries to mesh-aware versions
   (Spring / Quarkus / Go), ensure `SERVICE_MESH_TYPE` in Helm values / schema
   (may already be present from Step 1 — idempotent) and wire it into
   Deployment `env:`, and add the `httproutes-generator-maven-plugin` for Java
   services (building and committing its output when Maven is available).
3. Read `.mesh-migration/reports/mesh-build-wiring.yaml`. **If
   `status: partial`**, batch its `unresolved:` questions to the user in one
   round (e.g. `java-registration-artifact`: webclient or resttemplate?),
   deliver the answers via `resolutions` (continue the sub-agent or re-invoke),
   and re-read the report. Then copy `done:` / `skipped:` / `commandsRun:` /
   `forbiddenRouteErrors:` / `needsReview:` items into the log and the per-step
   status rows for 2.1, 2.2, and 2.3. `status: failed` → apply the
   [Error policy](#error-policy--read-before-executing-any-step).
4. **Forbidden routes (Java).** The `java-forbidden-routes` question means
   `httproutes-generator-maven-plugin` failed the build: paths forbidden (404)
   in legacy would be routed by Istio. **Stop and ask the user** — relay the
   question verbatim, with its three options and the recommended
   `split-controllers` (and its ⚠ BWC break: the gateway URLs of moved endpoints
   change). Do not pick an answer yourself and do not continue to Step 2.4 before
   the user answered. Deliver the answer via `resolutions`:
   - `forbidden-route-annotations` / `auto-generate-authorization-policies` →
     the sub-skill applies it and rebuilds; continue once its report is
     `complete`. Log the annotated files (or the plugin option) under **Done**.
   - `split-controllers` → the report comes back with
     `blockedBy: java-forbidden-routes`; apply
     [Stop for a controller refactor](#stop-for-a-controller-refactor).

#### Stop for a controller refactor

The user chose to split controllers, which only a developer can do. Then:

1. Log every `needsReview:` entry of the report under **Needs review**, and set
   status row 2.3 to `blocked — controller refactor`; rows 2.4–2.7 to
   `not run`.
2. Do **not** run Steps 2.4–2.7.
3. Print:
   > ⏸ Migration paused at Step 2.3: split the controllers listed in
   > `.mesh-migration/MIGRATION_LOG.md` so each class-level gateway prefix has
   > one route type. ⚠ This changes the gateway URLs of the moved endpoints —
   > update their clients. Then rerun this migration from Step 2.3.
4. Produce the [Final report](#final-checklist-and-hand-off) with the open
   items, and stop.

### Step 2.4 — Generate HTTPRoute CRs from route registration code

**Idempotency:** always invoke the sub-skill, even when
`source-code-httproutes.yaml` already exists — generation is deterministic, so
an unchanged source tree reproduces the same file, and the run produces the
report this step's follow-up items read.

1. Invoke sub-skill [`httproute-from-code`](../httproute-from-code/SKILL.md) with
   `interactive: false`, the source-code path, **and** the resolved `backendRefName` / `backendRefPort`
   (detected by Step 1, or asked from the user if still unresolved — propose the
   defaults `{{ .Values.DEPLOYMENT_RESOURCE_NAME }}` / `8080`). The sub-skill uses
   these for every generated `backendRefs[].name` / `backendRefs[].port` so the
   code-generated routes match the Maven-plugin output from Step 2.3.
   Also pass migration-wide `routeLabels` (detected in Step 1 or provided by user)
   so every generated HTTPRoute uses the same labels as declarative and plugin
   generated routes.
2. That skill scans Go (`*.go`) and Java (`*.java`) files, extracts
   `routeregistration.Route` / `RouteEntry` definitions, groups by `RouteType`,
   and emits one HTTPRoute CR per type to
   `helm-templates/<service name>/templates/source-code-httproutes.yaml`.
3. Read `.mesh-migration/reports/httproute-from-code.yaml`. **If
   `status: partial` with `route-conflict/*` entries**, relay them verbatim in
   one batch and deliver the answers via `resolutions`, as in Step 1.
   **If `status: partial` with an `unresolved:` entry `microservice-name`** (the
   output contains the literal `<microservice-name>` placeholder):
   - Ask the user for the service name and deliver it via `resolutions`
     (continue the sub-agent or re-invoke), then re-read the report.
   - If the user cannot provide one, do **not** leave the literal placeholder
     in place — it will break `helm template` silently. Rename the output file
     to `source-code-httproutes.yaml.incomplete`, add a prominent comment at
     the top (`# INCOMPLETE: replace <microservice-name> before committing`),
     and add a **Needs review** entry: "Microservice name could not be
     resolved — file renamed to `.incomplete`; set the correct name and rename
     before merging."
4. Do not make any inference for `source-code-httproutes.yaml` - besides what `httproute-from-code` skill does. 
5. **Commit all generated files** to the branch. Remind the user:
   > The generated `source-code-httproutes.yaml` must stay committed. Any time
   > route registration code changes, rerun the `httproute-from-code` skill and
   > commit the updated output before raising a PR.
6. Read `.mesh-migration/reports/httproute-from-code.yaml` and copy its
   `filesGenerated`, `routesGenerated`, `authorizationPoliciesGenerated`, and
   `needsReview` items into the log.
7. For every `needsReview` entry in the report (skipped rows, `ERROR:`
   sections), add a **Needs review** log entry.

### Steps 2.5–2.7 — Validate the result (delegate to `istio-migration-validate`)

1. Invoke the sub-skill
   [`istio-migration-validate`](../istio-migration-validate/SKILL.md) with
   inputs: `chartPath: <chart path>`, `repoRoot: <service root>` (see
   [Service root](#service-root-reporoot--do-not-ask-up-front)),
   `interactive: false`.
2. That skill will: verify every HTTPRoute file carries the Istio guard (adding
   missing guards — the one safe automatic fix), run the two `helm template`
   render checks (Istio mode produces HTTPRoutes/Gateways; Core mode leaks
   none), flag duplicate HTTPRoute rules (same parent + equal match), and flag
   imperative control-plane API calls in shell scripts and manifests — all
   without modifying anything else.
3. Read `.mesh-migration/reports/istio-migration-validate.yaml`; copy `guardsAdded:`
   (log under **Done**), `commandsRun:`, `controlPlaneCalls:` (log the count
   under **Done**), and `needsReview:` items into the log and the per-step
   status rows for 2.5, 2.6, and 2.7. Every `denyOverlaps` finding goes under
   **Needs review**. `status: failed` → apply the
   [Error policy](#error-policy--read-before-executing-any-step).

---

## Final checklist and hand-off

Before declaring the migration complete — or when it stops early — produce the
**Final report** and the plain-language closing summary exactly as
[`migration-log.md`](migration-log.md) → "Final checklist and hand-off"
describes.

---

## Operating rules

- **Never skip the log.** If the log file cannot be written, stop and report.
- **Never invent values.** Versions, package names, ports, microservice names —
  if unknown, add a **Needs review** entry instead of guessing.
- **Never guess an unresolved item.** When a sub-skill report says
  `status: partial`, batch its `unresolved:` questions to the user in one round
  and re-invoke with the answers — do not infer gateway types or names yourself.
- **Never run destructive commands.** Do not push, tag, or delete branches. Do
  not modify git config.
- **Be explicit in chat.** After each step, print a one-line summary plus the
  updated per-step status row.
- **Idempotent reruns.** Rerunning a step must not duplicate or corrupt its
  output. For delegated steps the sub-skill owns that check and logs
  already-present items under **Done** — always invoke it anyway, so its report
  is on disk for the steps that read it.
- **Follow the Error policy.** On any unrecoverable failure, stop the step, log
  it, and ask the user whether to continue or abort before taking further action.

---

## Non-goals

This skill only modifies:
Helm templates, `values.yaml`, `values.schema.json`, `pom.xml`, `go.mod`, the
consumer `.gitignore` (the `.mesh-migration/` entry), files under
`.mesh-migration/` (reports, worksheets and `MIGRATION_LOG.md`), and — only when
the user answers `forbidden-route-annotations` — Java sources, where it adds
`@ForbiddenRoute` annotations and their imports.

It does not raise pull requests, push branches, rewrite application logic (it
never splits controllers itself), generate `VirtualService` resources, or modify
git configuration.

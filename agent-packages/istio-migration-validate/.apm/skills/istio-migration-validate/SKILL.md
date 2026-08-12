---
name: istio-migration-validate
description: >
  Validate the output of a Core Mesh to Istio migration: verify every HTTPRoute
  file is wrapped in the SERVICE_MESH_TYPE=Istio guard (adding missing guards),
  check that no HTTPRoute renders under Core mode, flag duplicate HTTPRoute
  rules (same parent + equal match), and flag imperative control-plane API calls
  in shell scripts and manifests. Use when asked to validate or check a
  migrated chart, or as a sub-skill of core-mesh-to-istio-migration.
---

# Istio migration validation

## Contract

### Inputs

| Input | Type | Required | Notes |
|---|---|---|---|
| `chartPath` | path | yes | Helm chart to validate |
| `repoRoot` | path | no | Search root for the Step 4 scan; defaults to the repository containing `chartPath` |
| `interactive` | bool | no | `true` only when a user invokes the skill directly; orchestrators and sub-agent wrappers pass `false` |

With `interactive: false` (the default for orchestrated and sub-agent runs),
never ask the user: every blocking question becomes an `unresolved:` entry and
the report finishes with `status: partial` (or `failed` if a render check
fails). With `interactive: true`, ask blocking questions in chat and wait.

If `chartPath` is missing: with `interactive: true`, ask before starting; with
`interactive: false`, add `unresolved:` id `chartPath` and stop with
`status: partial`.

### Outputs

In addition to the chat summary, write a machine-readable report to
`.mesh-migration/reports/istio-migration-validate.yaml` (create the directory, and ensure
`.mesh-migration/` is listed in the repo's `.gitignore` — reports are working
files, never committed; the orchestrator handles both in orchestrated runs):

```yaml
reportSchema: 1
skill: istio-migration-validate
status: complete            # complete | partial | failed (a render check failed)
generatedAt: <ISO-8601>
guardsAdded: [<files that were missing the Istio guard and got it>]
renderChecks:
  istioMode: pass | fail    # HTTPRoute/Gateway present under SERVICE_MESH_TYPE=Istio
  coreMode: pass | fail     # no HTTPRoute leaks under SERVICE_MESH_TYPE=Core
duplicateGroups: <N>
controlPlaneCalls: <N>        # imperative control-plane calls found by Step 4
commandsRun:
  - command: <cmd>
    exitCode: <N>
unresolved: []
needsReview:
  - <one line per finding: leaked file, duplicate group, unguardable file,
     control-plane call>
```

Consumers must ignore unknown report fields. A consumer that sees a
`reportSchema` newer than its own documentation must stop and report a contract
mismatch instead of guessing field meanings.

### Side effects

The **only** file modification this skill may make is adding a missing
`{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}` … `{{- end }}` guard (Step 1).
Everything else is read-only reporting.

---

## Step 1 — Verify all HTTPRoutes are wrapped in Istio conditionals

1. List every file under `<chartPath>/templates/` (including generated
   `-istio.yaml` / `annotations-httproutes.yaml` / `source-code-httproutes.yaml`)
   that contains `kind: HTTPRoute`.
2. For each file, confirm the HTTPRoute block is inside a single
   `{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}` … `{{- end }}`. If a file
   has multiple HTTPRoute documents, the guard must wrap the whole block with
   `---` separators kept inside.
3. If a file is missing the guard → add it (the one safe automatic fix) and
   record the file under `guardsAdded:`. If a guard cannot be added safely
   (mixed guarded/unguarded documents), add a `needsReview:` entry instead.

## Step 2 — Render checks

```bash
helm dependency update

# Must return at least one HTTPRoute or Gateway line
helm template <chartPath> --set SERVICE_MESH_TYPE=Istio \
  | grep -E 'kind: (HTTPRoute|Gateway)'

# Must return nothing — HTTPRoutes must not leak under Core mode
helm template <chartPath> --set SERVICE_MESH_TYPE=Core \
  | grep 'kind: HTTPRoute'
```

Record each command and exit code under `commandsRun:` and the outcomes under
`renderChecks:`.

- Istio mode returns no HTTPRoute/Gateway lines → `istioMode: fail`,
  `status: failed`, and a `needsReview:` entry.
- Core mode returns any HTTPRoute lines → `coreMode: fail`, `status: failed`,
  and one `needsReview:` entry per offending file path.

## Step 3 — Detect duplicate HTTPRoute rules

After all HTTPRoutes exist (declarative conversions, Maven-plugin annotations,
code generation), the same route can be emitted twice — e.g. a route declared in
a `RouteConfiguration` CR **and** registered in code. Duplicate rules with the
same parent and identical match are ambiguous and **must not be auto-removed**
(deleting the wrong one can change runtime behavior). Flag them instead.

1. Collect every HTTPRoute rule across all files guarded by
   `SERVICE_MESH_TYPE=Istio` (`-istio.yaml`, `annotations-httproutes.yaml`,
   `source-code-httproutes.yaml`, and any inline HTTPRoutes).
2. For each rule, build a comparison key from:
   - the **parent** — every `parentRefs[]` entry it belongs to
     (`group` + `kind` + `name`), and
   - the **match parameters** — the normalized `matches[]`: path `type` + `value`,
     plus any `headers[]` / `queryParams[]` / `method`.
   Resolve `{{ .Values.* }}` expressions textually (compare the literal template
   string); do not attempt to render Helm.
3. A **duplicate** is two or more rules that share **at least one common parent**
   AND have an **equal match key**. Rules that share a match but target only
   different parents are NOT duplicates.
4. For every duplicate group, add **one** `needsReview:` entry containing:
   - the shared parent(s) and the match value,
   - every file + HTTPRoute `metadata.name` (and rule index) that contributes a
     copy,
   - suggested action: "Two routes resolve to the same parent and match —
     confirm which source is authoritative and remove the redundant rule (often
     a route present both in a `RouteConfiguration` CR and in registration
     code)."
5. **Do not modify any file in this step.** It only reports. Record the group
   count under `duplicateGroups:`.

## Step 4 — Flag imperative control-plane calls

Routes and other mesh config can also be pushed to the Cloud-Core
Control-Plane **imperatively** — a `curl`/`wget` against the control-plane
REST API from a shell script, a Helm hook, a Kubernetes `Job`, an
init-container, or a lifecycle command. These calls bypass the declarative
CRs, the route-registration libraries, and the `SERVICE_MESH_TYPE` guard
entirely, so the migration steps never convert them. Under Istio the
control-plane no longer routes this traffic, so every such call needs a
human decision (drop it, or replace it with an equivalent HTTPRoute).

**This step only reports. Never edit, delete, or rewrite a script — none of
these calls are safe to auto-migrate.**

1. Search `<repoRoot>` for shell that talks to the control-plane. Cover both
   standalone scripts and shell embedded in manifests:
   - files matching `*.sh` / `*.bash`,
   - `command:` / `args:` shell in Kubernetes `Job`, `CronJob`,
     init-containers, and container `lifecycle` hooks,
   - Helm hook resources (`helm.sh/hook` annotations) that run scripts.

   ```bash
   grep -rniE 'control-plane|controlplane|/api/v[0-9]+/(routes|control-plane)' \
     --include='*.sh' --include='*.bash' --include='*.yaml' --include='*.yml' <repoRoot>
   ```

   Treat any `curl`/`wget`/HTTP client call whose target host or path
   references the control-plane (e.g. `control-plane:8080`,
   `/api/v3/routes/...`, registration/blue-green endpoints) as a hit. When
   in doubt, flag it — false positives cost a human a glance, a missed call
   silently breaks routing after cutover. Record the grep command and its
   exit code under `commandsRun:` (same audit trail as the Step 2 helm checks).
2. For **every** hit, add **one** `needsReview:` entry containing:
   - the file (and line) of the call,
   - the control-plane endpoint and HTTP method invoked,
   - suggested action: "Imperative control-plane call not covered by the
     migration — under Istio the control-plane does not route this traffic.
     Confirm the route is now served by an HTTPRoute (declarative or
     generated), then remove this call — or, if the chart must keep working in
     Core mode, gate it on `SERVICE_MESH_TYPE=Core`. Do not leave it firing
     unconditionally against an Istio environment."

   Spell the gate out in the entry, picking the form that fits where the call
   lives:
   - **Helm-templated manifest** (`Job`, `CronJob`, init-container,
     `lifecycle` hook, hook resource) — wrap it in
     `{{- if eq .Values.SERVICE_MESH_TYPE "Core" }}` … `{{- end }}`: the guard
     Step 1 verifies on HTTPRoutes, with the mesh type flipped to `Core`.
   - **Standalone shell script** — exit early unless the mesh type is Core,
     e.g. `[ "$SERVICE_MESH_TYPE" = "Core" ] || exit 0`. Note in the entry that
     the script's container must receive the `SERVICE_MESH_TYPE` env var; the
     Deployment wiring does not cover `Job` or hook pods.
3. Record the hit count under `controlPlaneCalls:` (`0` when the scan finds
   nothing). The scan reads the working tree on every run, so a rerun
   re-derives its findings; there is nothing to skip.

---

## Output

Write the contract report file (see [Contract → Outputs](#outputs)), then print
a short chat summary: guards added, render-check outcomes, duplicate groups
found, control-plane calls found, and every `needsReview:` entry.

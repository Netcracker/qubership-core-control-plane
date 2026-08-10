---
name: istio-migration-validate
description: >
  Validate the output of a Core Mesh to Istio migration: verify every HTTPRoute
  file is wrapped in the SERVICE_MESH_TYPE=Istio guard (adding missing guards),
  check that no HTTPRoute renders under Core mode, and flag duplicate HTTPRoute
  rules (same parent + equal match). Use when asked to validate or check a
  migrated chart, or as a sub-skill of core-mesh-to-istio-migration.
---

# Istio migration validation

## Contract

### Inputs

| Input | Type | Required | Notes |
|---|---|---|---|
| `chartPath` | path | yes | Helm chart to validate |

If invoked standalone and `chartPath` is missing, ask the user before starting.

### Outputs

In addition to the chat summary, write a machine-readable report to
`.migration/reports/istio-migration-validate.yaml` (create the directory; the
path is gitignored):

```yaml
reportSchema: 1
skill: istio-migration-validate
status: complete            # complete | failed (a render check failed)
generatedAt: <ISO-8601>
guardsAdded: [<files that were missing the Istio guard and got it>]
renderChecks:
  istioMode: pass | fail    # HTTPRoute/Gateway present under SERVICE_MESH_TYPE=Istio
  coreMode: pass | fail     # no HTTPRoute leaks under SERVICE_MESH_TYPE=Core
duplicateGroups: <N>
commandsRun:
  - command: <cmd>
    exitCode: <N>
unresolved: []
needsReview:
  - <one line per finding: leaked file, duplicate group, unguardable file>
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

---

## Output

Write the contract report file (see [Contract → Outputs](#outputs)), then print
a short chat summary: guards added, render-check outcomes, duplicate groups
found, and every `needsReview:` entry.

---
name: istio-migration-validate
description: >
  Validate the output of a Core Mesh to Istio migration: verify every Istio /
  Gateway API resource file is wrapped in the SERVICE_MESH_TYPE=Istio guard
  (adding missing guards), check that no Istio resource renders under Core mode,
  flag duplicate HTTPRoute rules (same parent + equal match), lint for leftover
  no-backend/regex rules and missing public-gateway AuthorizationPolicy, and
  flag imperative control-plane API calls in shell scripts and manifests. Use
  when asked to validate or check a migrated chart, or as a sub-skill of
  core-mesh-to-istio-migration.
---

# Istio migration validation

## Contract

### Inputs

| Input | Type | Required | Notes |
|---|---|---|---|
| `chartPath` | path | yes | Helm chart to validate |
| `repoRoot` | path | no | Service root for the Step 4 scan (chart + its scripts). Defaults to the service/module root that owns `chartPath` — not a multi-service monorepo git root |
| `interactive` | bool | no | `true` only when a user invokes the skill directly; orchestrators and sub-agent wrappers pass `false` |
| `publicPolicyDecision` | string | no | `none-required` only after the owner confirmed no public endpoints and effective public-gateway access was checked. Omit otherwise |

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
chartPath: <chartPath>
status: complete            # complete | partial | failed (a render check failed)
generatedAt: <ISO-8601>
guardsAdded: [<files that were missing the Istio guard and got it>]
renderChecks:
  istioMode: pass | fail | skipped   # Istio resources present under SERVICE_MESH_TYPE=Istio
  coreMode: pass | fail | skipped    # no Istio resources leak under SERVICE_MESH_TYPE=Core
duplicateGroups: <N>
oversizedHttpRoutes: <N>      # HTTPRoutes with >16 rules (Gateway API CRD maxItems: 16)
accessControlLint:            # Step 3b — read-only, additive
  noBackendRules: <N>          # rules with neither backendRefs nor a RequestRedirect filter
  publicPolicyDecision: <none-required | omitted>
  regexMatches: <N>            # matches[].path.type: RegularExpression still present
  publicGatewayPolicies: <N>   # AuthorizationPolicies targeting Gateway public-gateway
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

## Step 1 — Verify all Istio / Gateway API resources are wrapped in Istio conditionals

1. List every file under `<chartPath>/templates/` (including generated
   `-istio.yaml` / `annotations-httproutes.yaml` / `source-code-httproutes.yaml` /
   `public-gateway-authorization-policy.yaml`) that contains an Istio or Gateway API resource,
   i.e. an `apiVersion:` starting with `gateway.networking.k8s.io/`, `networking.istio.io/`,
   `security.istio.io/` or `extensions.istio.io/` (HTTPRoute, Gateway, DestinationRule,
   ServiceEntry, AuthorizationPolicy, TrafficExtension). Do **not** match on `kind: Gateway` alone:
   legacy Core Mesh Gateways use `nc.core.mesh/*` and belong under the Core guard.
2. For each file, confirm those resources are inside a single
   `{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}` … `{{- end }}`. If a file
   has multiple such documents, the guard must wrap the whole block with
   `---` separators kept inside.
3. If a file is missing the guard → add it (the one safe automatic fix) and
   record the file under `guardsAdded:`. If a guard cannot be added safely
   (mixed guarded/unguarded documents), add a `needsReview:` entry instead.

## Step 2 — Render checks

If `helm` is not installed: record under `commandsRun:` as skipped, set both `renderChecks` to
`skipped`, add a `needsReview:` entry "helm not available — run the two render checks by hand"
and continue with the next step (not a failure).

Each run uses a private temporary directory so a second validation cannot overwrite this run's
render. Require each `helm template` to exit 0 before scanning the file it wrote. Delete the
directory after the checks, including when a render fails.

```bash
render_dir=$(mktemp -d)
helm dependency update <chartPath>                                                # must exit 0
helm template <chartPath> --set SERVICE_MESH_TYPE=Istio > "$render_dir/istio-render.yaml"  # must exit 0
helm template <chartPath> --set SERVICE_MESH_TYPE=Core  > "$render_dir/core-render.yaml"   # must exit 0

# Scan only this run's files, and only after that mode's helm template exited 0.
# Istio mode: must find at least one match (grep exit 0)
grep -E '^apiVersion: (gateway\.networking\.k8s\.io|networking\.istio\.io|security\.istio\.io|extensions\.istio\.io)/' "$render_dir/istio-render.yaml"

# Core mode: must find NONE. grep exit code 1 (no match) is the PASSING result.
grep -E '^apiVersion: (gateway\.networking\.k8s\.io|networking\.istio\.io|security\.istio\.io|extensions\.istio\.io)/' "$render_dir/core-render.yaml"
rm -rf "$render_dir"
```

Do not write `/tmp/istio-render.yaml` or `/tmp/core-render.yaml`. Do not pipe `helm template` into
`grep`: a failed render can look like an empty result.

Record each command and exit code under `commandsRun:` and the outcomes under
`renderChecks:`.

- A `helm template` command exits non-zero (chart needs extra values, broken template) →
  `status: failed`, quote its error under `needsReview:`; do not evaluate that mode's grep.
  Still delete `render_dir`.
- Istio mode: no match → `istioMode: fail`, `status: failed`, and a `needsReview:` entry.
- Core mode: any match → `coreMode: fail`, `status: failed`, and one `needsReview:` entry per
  offending resource (kind + name; locate its source file under `<chartPath>/templates/`).

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
     confirm which source is authoritative and remove the redundant rule. Typical
     causes: a route present both in a `RouteConfiguration` CR and in registration
     code, or separate public/private/internal `RouteConfiguration` CRs that were all
     widened to the same three gateways."
5. **Do not modify any file in this step.** It only reports. Record the group
   count under `duplicateGroups:`.

## Step 3b — Access-control lint

Per [docs/istio/allowed-and-forbidden-routes.md](https://github.com/Netcracker/qubership-core-control-plane/blob/main/docs/istio/allowed-and-forbidden-routes.md),
Istio has no forbidden-route (404-without-backend) rule and no per-gateway deny — access control
for the public gateway is an `AuthorizationPolicy`, normally written by hand (see
[`public-gateway-authz-policy`](../public-gateway-authz-policy/SKILL.md)). This step only reports;
it never edits or generates anything.

1. **No-backend rules.** Across every HTTPRoute file guarded by `SERVICE_MESH_TYPE=Istio`, count
   rules that have neither `backendRefs` nor a `RequestRedirect` filter (`type: RequestRedirect`
   or a `requestRedirect` filter). A Gateway API redirect intentionally has no backend; do not
   flag it. A remaining rule with no backend has no Istio equivalent of the legacy forced-404
   route and usually means an older, unmigrated conversion slipped through. `noBackendRules` is
   this count, not the count of every rule that lacks `backendRefs`. Record it under
   `accessControlLint.noBackendRules:`; for each flagged rule, add a `needsReview:` entry naming
   the file and rule.
2. **Leftover regex matches.** Count `matches[].path.type: RegularExpression` occurrences across
   the same files — generated and migrated routes should all be `PathPrefix`/`Exact` now (see
   `regExp reduction` in
   [`core-mesh-crs-to-istio/route-configuration-mapping.md`](../core-mesh-crs-to-istio/route-configuration-mapping.md)
   and `Step 6a` in [`httproute-from-code/SKILL.md`](../httproute-from-code/SKILL.md)). Record the
   count under `accessControlLint.regexMatches:`; for each one, add a `needsReview:` entry naming
   the file and rule (a header-matcher `RegularExpression` is not in scope here — only path
   matches).
3. **Public-gateway AuthorizationPolicy presence.** Count `AuthorizationPolicy` resources in the
   chart whose `spec.targetRefs` includes `{kind: Gateway, name: public-gateway}`. Record the
   count under `accessControlLint.publicGatewayPolicies:`.
   - **`0`** and `publicPolicyDecision` is `none-required` → do not add a finding. Record
     `accessControlLint.publicPolicyDecision: none-required`. The caller may pass that value only
     after effective Gateway permissions have been checked. A project-level ALLOW can still make
     an attached route public; this step does not reopen a decision the owner already made.
   - **`0`** otherwise → add a `needsReview:` entry: "No public-gateway `AuthorizationPolicy`
     found — if this service must be reachable from outside the cluster, write one by hand (see
     `public-gateway-authz-policy`). Absence of a service-owned policy is not proof every route is
     blocked: check other ALLOW policies that target `public-gateway`, including project policies
     outside this chart. Services with no public endpoints need none only after that check."
   - **`1` or more** → no finding; this step does not evaluate whether the policy's paths are
     correct, only that one exists.
4. **16-rule limit.** Across every Istio-guarded `HTTPRoute`, check `len(spec.rules) <= 16`
   (Gateway API CRD `maxItems: 16` — Kubernetes rejects CRs with more). Record the offending count
   under `oversizedHttpRoutes:`; for each one, add a `needsReview:` entry naming the file,
   `HTTPRoute` name, and rule count, with suggested action: "Split into separate `HTTPRoute` CRs
   of at most 16 rules each (`<name>-1`, `<name>-2`, …)."
5. **Old parent mapping.** In `*-istio.yaml`, `source-code-httproutes.yaml`, and
   `annotations-httproutes.yaml`, an HTTPRoute still uses the old parent mapping when its only
   parent is `public-gateway`, `private-gateway`, or `internal-gateway-service`. Add one
   `needsReview:` entry: "HTTPRoute `<name>` still has the old parent mapping (`<parents>`).
   Reconcile with core-mesh-crs-to-istio `reconcileParents: true`, which rewrites only
   `spec.parentRefs` and preserves consumer edits, or regenerate. Add-missing-only does not
   refresh parents." A single non-border Service parent is a mesh route, not this finding.
   Do not edit parents in this step.
6. **Do not modify any file in this step.** It only reports.

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
     --exclude-dir=.mesh-migration --exclude-dir=.git --exclude-dir=target \
     --include='*.sh' --include='*.bash' --include='*.yaml' --include='*.yml' <repoRoot>
   ```

   Treat any `curl`/`wget`/HTTP client call whose target host or path
   references the control-plane (e.g. `control-plane:8080`,
   `/api/v3/routes/...`, registration/blue-green endpoints) as a hit. When
   in doubt, flag it — false positives cost a human a glance, a missed call
   silently breaks routing after cutover. Record the grep command and its
   exit code under `commandsRun:`; exit code `1` simply means "no hits" and is not an error.
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
found, oversized `HTTPRoute` CRs (>16 rules), access-control lint results (no-backend rules, leftover regex matches,
public-gateway `AuthorizationPolicy` count), control-plane calls found, and
every `needsReview:` entry.

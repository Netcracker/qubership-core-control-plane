---
name: core-mesh-to-istio-migration
description: >
  Orchestrate the full Cloud-Core Mesh to Istio migration end-to-end — convert
  declarative mesh CRs to Gateway API, migrate route-registration libraries, wire
  SERVICE_MESH_TYPE, add the Java HTTPRoute generator, generate HTTPRoutes from
  Go/Java code, recommend (and, on request, draft) the public-gateway
  AuthorizationPolicy, validate Istio guards / duplicate rules / access control /
  imperative control-plane calls, and maintain .mesh-migration/MIGRATION_LOG.md.
  Use when asked to migrate a service from Core Mesh to Istio (Ambient Mesh) or run
  the migration guide end-to-end.
---

# Core Mesh → Istio — Full Migration Orchestrator

This skill is an **orchestrator**: the work lives in five atomic sub-skills; this skill
coordinates them, asks the user only what cannot be discovered, and keeps an auditable log.
The user may know nothing about Istio or Core Mesh — explain every question in one plain sentence.

## Invocation

Run against the chart or service directory to migrate (`helm-templates/my-service`, `.`). The user
may name a **start step** ("start from Step 2.4", "rerun the validation") to resume; the default is a
full run from Step 1.

Saved prompts from before the policy step was inserted use the old validation numbers. Map them
before choosing a start step:

| What the user says | Start at |
|---|---|
| "rerun the validation", "run istio-migration-validate" | Step 2.6 |
| old Step 2.5 (guards and render checks) | Step 2.6 |
| old Step 2.6 (duplicate rules) | Step 2.7 |
| old Step 2.7 (control-plane calls) | Step 2.8 |

Current Step 2.5 is public-gateway policy authoring. Do not open it for a request to rerun
validation.

## Sub-skills

| Sub-skill | Steps | Purpose |
|---|---|---|
| [`core-mesh-crs-to-istio`](../core-mesh-crs-to-istio/SKILL.md) | 1 | Convert Helm mesh CRs to Gateway API / Istio resources |
| [`mesh-build-wiring`](../mesh-build-wiring/SKILL.md) | 2.1–2.3 | Mesh-aware libraries, `SERVICE_MESH_TYPE`, Maven plugin |
| [`httproute-from-code`](../httproute-from-code/SKILL.md) | 2.4 | HTTPRoutes from Go/Java route-registration code |
| [`public-gateway-authz-policy`](../public-gateway-authz-policy/SKILL.md) | 2.5 | Recommend (draft only on explicit request) the public-gateway AuthorizationPolicy |
| [`istio-migration-validate`](../istio-migration-validate/SKILL.md) | 2.6–2.8 | Guards, render checks, duplicates, access-control lint, control-plane calls |

**How to invoke a sub-skill:** pass resolved inputs and always `interactive: false`; sub-skills never
talk to the user — their questions come back as `unresolved:` report entries. If the harness has a
sub-agent/Task tool, run the sub-skill in a sub-agent; otherwise read its `SKILL.md` in full and
execute it inline. Either way take results from its **report file**, not its transcript.

## Sub-skill reports

Reports live in `.mesh-migration/reports/<skill>.yaml`.

- **Before the first step**, ensure the repo `.gitignore` contains `.mesh-migration/` (append it with
  a short comment if missing; log under **Done**). Reports and the log are never committed.
- **Full run:** delete `.mesh-migration/reports/` and any old `MIGRATION_LOG.md` first.
- **Resumed run:** keep existing reports only when each reused report has `status: complete` and
  `chartPath` equal to the selected chart. Before executing, list each reused report (`skill`,
  `generatedAt`, `status`, `chartPath`) and say: "Resuming from <step>; reusing these previous
  results — rerun from Step 1 instead if they are stale." A report with no `chartPath`, or a
  different one, cannot be tied to this chart: do not reuse it. Ask for a fresh run of that
  producer or for explicit access-control input. Producer reports (`core-mesh-crs-to-istio`,
  `httproute-from-code`) also need `accessControl.collected: true`. A missing `collected` flag is
  an older inventory, not an empty one — re-invoke that producer before Step 2.5 recommends or
  drafts a policy. A `partial` report is not reusable as-is. Apply the same resolution flow as a
  fresh partial report: relay every `unresolved:` question, deliver `resolutions`, re-invoke, and
  require `status: complete` before any later step reads that producer's candidates, `backendRef`,
  or `labels`. A route with an unresolved Gateway must not stay unconverted while Step 2.5 runs.
  If a report needed later is missing or `failed`, ask the user to start earlier or provide the
  values. Mark skipped steps `reused (report of <generatedAt>)` only after those checks pass.
- **Always invoke every sub-skill of the steps you run** when its report cannot be reused. When it
  can, the report file — not a transcript — is what later steps read.
- After each sub-skill, read its report; ignore unknown fields. If `reportSchema` is missing or newer
  than that skill's Contract documents, stop and add a **Needs review** entry (contract mismatch).
- `status: partial` → relay every `unresolved:` entry's `question` (and `options`) to the user in
  **one** batch, deliver answers as a `resolutions` map keyed by entry `id` (continue the same
  sub-agent, or re-invoke with `resolutions`), then re-read the report and confirm
  `status: complete`. Repeat at most twice; if still `partial`, log each leftover entry under
  **Needs review** and ask the user whether to continue or abort.

---

## Step 0 — Discover inputs (ask only what you cannot find)

Look in the repo yourself first:

1. **Chart path** — directory with `Chart.yaml` (usually `helm-templates/<name>` or `charts/<name>`).
2. **Language and build** — `pom.xml` → Java (Spring if `spring-boot-*`, Quarkus if `quarkus-*`);
   `go.mod` → Go; both possible. `build.gradle*` → Java/Gradle (the Maven plugin steps are skipped
   and logged under **Needs review**).
3. **Source path** — the directory holding `pom.xml`/`go.mod` (often `.`).

Print a 4-line summary (chart, source path, language, build) and ask the user to confirm or correct
it. Ask a real question only when something is missing or ambiguous (several charts, no `Chart.yaml`).
Never guess the chart path.

**`repoRoot`** for `istio-migration-validate` = the directory that owns both the chart and its
scripts; prefer the source path when it is the service root. Never widen to a multi-service
monorepo root.

### Backend reference (`backendRefName` / `backendRefPort`) — do not ask up front

1. Step 1 detects them from the existing `RouteConfiguration` destinations (report field `backendRef`).
2. If both are detected: log them under **Done** and reuse them in Steps 2.3 and 2.4 without asking.
3. Otherwise, at the point they are first needed (Step 2.3 for Java, else 2.4), propose the defaults
   `{{ .Values.SERVICE_NAME }}` and `8080` and ask for confirmation. Before asking, look at the
   chart's own Service template — if it names a different Service, propose that name instead.

Use the same values for the Maven plugin (`<backendRefVal>`, `<servicePort>`) and `httproute-from-code`.

### Route labels (`routeLabels`)

Use Step 1's report `labels.values` map (not the whole `labels` object) as migration-wide
`routeLabels` for Steps 2.3 and 2.4. If `labels.unresolvedReason` is set, do not invent values:
propose the standard set — `app.kubernetes.io/name: {{ .Values.SERVICE_NAME }}`,
`app.kubernetes.io/part-of: {{ .Values.APPLICATION_NAME }}`,
`app.kubernetes.io/managed-by: {{ .Values.MANAGED_BY }}`,
`deployment.netcracker.com/sessionId: {{ .Values.DEPLOYMENT_SESSION_ID }}`,
`deployer.cleanup/allow: "true"`, `app.kubernetes.io/processed-by-operator: istiod` — and ask the
user to accept it or give another map, when first needed.

---

## Error policy

On an unrecoverable error (non-zero exit of a required command, a sub-skill `status: failed`, a file
that cannot be written):

1. Stop the current step; do not start the next one.
2. Log it under **Needs review** (error, failing file/command, suggested fix).
3. Tell the user: "⛔ Step `<N>` failed: `<one-line error>`. Reply **continue** to skip this step or
   **abort** to stop." Wait for the reply.

Optional steps (e.g. `mvn` not installed) are logged under **Skipped** without asking.

## Migration log — MANDATORY

Maintain `.mesh-migration/MIGRATION_LOG.md`, updated **after every step**. If it cannot be written,
stop and report. A full run starts a fresh log; a resumed run appends.

````markdown
# Core Mesh → Istio Migration Log
Started: <ISO-8601>   Chart: <path>   Code: <path>   Language: <Go | Java | Go+Java>

## Done
**Items fully applied by automation, one bullet per concrete change.**
## Skipped
**Items intentionally not applied, with reason.**
## Needs review
**Items the user MUST verify before merging. Each entry: file/location, why, suggested action.**
## Per-step status
| Step | Title | Status | Notes |
|------|-------|--------|-------|
| 1    | Migrate mesh CRs → HTTPRoute CRs            | pending | |
| 2.1  | Switch to mesh-aware route libraries        | pending | |
| 2.2  | Set SERVICE_MESH_TYPE env var               | pending | |
| 2.3  | Add Maven plugin (Java only)                | pending | |
| 2.4  | Generate HTTPRoute CRs from code            | pending | |
| 2.5  | Public-gateway AuthorizationPolicy          | pending | |
| 2.6  | Istio guards and render checks              | pending | |
| 2.7  | Duplicate rules and access-control lint     | pending | |
| 2.8  | Imperative control-plane calls              | pending | |
## Commands run
| Step | Command | Exit code | Notes |
|------|---------|-----------|-------|
````

Rules: append concrete paths, names, counts and every command with its exit code; classify every
non-trivial action as Done / Skipped / Needs review; never overwrite the log or delete a Needs-review
entry until the user confirms it is resolved; after each step print a one-line chat summary
(`Updated MIGRATION_LOG.md — 3 done, 1 needs review`).

Copy every sub-skill `needsReview:` line and every `# ⚠ MANUAL REVIEW` hit into **Needs review**
**once** (do not log the same finding from both the report and the YAML comment). Values you cannot
infer (unresolved gateway, `<microservice-name>` placeholder, ambiguous Java artifact, unknown
library version) also go to **Needs review** — never guess them.

Copy each `accessControl.regexReductions` entry into the log: source rule, original expression,
and replacement match. Put a `conflicts: false` entry under **Done**. Put a `conflicts: true`
entry under **Needs review** only through the producer's `needsReview` line, not a second time.

---

## Execution plan

Run steps in order from the start step. After each: update the log and status row, print one line.

### Step 1 — Migrate declarative mesh CRs

1. Invoke [`core-mesh-crs-to-istio`](../core-mesh-crs-to-istio/SKILL.md) with `chartPath`,
   `interactive: false`. It guards the originals for Core, generates Istio `-istio.yaml` siblings
   (HTTPRoute, Gateway, DestinationRule, ServiceEntry, TrafficExtension, …), and updates
   `values.yaml` / `values.schema.json`.
2. `status: partial` → ask the user all `unresolved:` questions in one batch. For a gateway question
   explain: "*ingress* = an entry point that receives traffic from outside; *mesh* = internal
   service-to-service traffic. If unsure, answer mesh." Deliver `resolutions`, re-read the report.
3. Copy `filesModified` / `filesGenerated`, resource counts, `regexReductions`, and `needsReview`
   into the log. A `# ⚠ MANUAL REVIEW` flag is never safe to auto-fix: leave the comment in the
   file. If `publicAttachment.withheld` is non-empty, explain that those routes are not on
   `public-gateway` yet. Ask the user to check AuthorizationPolicies that target
   `public-gateway`, including project policies outside the chart
   ([crs-example.yaml](https://github.com/Netcracker/qubership-core-control-plane/blob/main/docs/istio/crs-example.yaml)
   section 3a). Absence of a policy in this chart is not proof the routes are unreachable. Only
   when the user confirms no ALLOW matches, re-invoke with `publicGatewayAccess: clear` and
   `reconcileParents: true`. If an in-chart ALLOW matches, leave the routes off `public-gateway`.
4. Capture `backendRef` and `labels.values` as described in Step 0.

### Steps 2.1–2.3 — Build and deployment wiring

Applies to services that use route-registration libraries or annotations. Invoke
[`mesh-build-wiring`](../mesh-build-wiring/SKILL.md) with `codePath`, `chartPath`, `language`,
`interactive: false` and, for Java, `backendRefName` / `backendRefPort` / `routeLabels`. It switches
libraries to mesh-aware versions, wires `SERVICE_MESH_TYPE`, and adds the matching Maven plugin
(`httproutes-generator-spring-maven-plugin` for Spring, `httproutes-generator-maven-plugin` for
Quarkus). Do not restate the Quarkus `routes-registrator` floor here. Copy
`versions.routesRegistrator` from the mesh-build-wiring report into the log — that field is the
version the child actually applied. Handle `partial` (e.g.
`java-registration-artifact`, `maven-plugin-packages`) via `resolutions`; copy `done:` /
`skipped:` / `commandsRun:` / `needsReview:` into the log and the 2.1–2.3 rows. `failed` → Error
policy.

### Step 2.4 — HTTPRoutes from route-registration code

Invoke [`httproute-from-code`](../httproute-from-code/SKILL.md) with the source path, `chartPath`,
`backendRefName`, `backendRefPort`, `routeLabels`, `interactive: false`. Finding **zero**
code-registered routes is normal (annotation-only Java services); log it under **Skipped**.

- `status: partial` with `microservice-name` → ask the user for the service name and deliver it via
  `resolutions`. If they cannot answer, rename the output to `source-code-httproutes.yaml.incomplete`,
  put `# INCOMPLETE: replace <microservice-name> before committing` at its top, and add a
  **Needs review** entry. Never leave the placeholder in a live template.
- Make no inferences beyond what the sub-skill did.
- Do **not** run `git add` / `git commit`. Remind the user: generated files must be committed, and
  `httproute-from-code` (and the Maven build for Java) must be rerun whenever route code changes.
- Copy `filesGenerated`, `routesGenerated`, `needsReview` into the log.

### Step 2.5 — Public-gateway access (AuthorizationPolicy)

Per [docs/istio/allowed-and-forbidden-routes.md](https://github.com/Netcracker/qubership-core-control-plane/blob/main/docs/istio/allowed-and-forbidden-routes.md),
routes that were already public are also registered on the private and internal gateways.
Formerly non-public routes stay off `public-gateway` until Step 1's attachment gate is clear.
**Recommend that the service owner write the AuthorizationPolicy by hand.** Never generate one
unless told to. Do not treat a missing service-owned policy as proof that a route is unreachable.

1. Use the Step 1 and Step 2.4 reports only when each has `chartPath` equal to the selected chart
   and `accessControl.collected: true`. Otherwise re-invoke the producer, or stop and ask for
   explicit access-control input. Do not merge a stale or foreign report.
2. Merge the accepted `accessControl` blocks. Keep `collected: true` and `chartPath` on the merged
   object. Invoke [`public-gateway-authz-policy`](../public-gateway-authz-policy/SKILL.md) with
   `chartPath`, `codePath`, the merged `accessControl`, `routeLabels`, `generate: false`,
   `interactive: false`.
3. Read its report. If `inventory` is `rejected` or `status` is `failed`, follow the Error policy.
   Do not describe the result as an empty candidate list.
4. If `existingPolicy` is set: log the path under **Done**, relay the candidates for comparison,
   and skip the question.
5. Otherwise relay the recommendation and candidate lists, including that a project-level ALLOW
   can cover paths the service does not list, and ask **once**: "I recommend you write the
   public-gateway AuthorizationPolicy yourself. Do you want me to generate a draft anyway?"
6. Branch on the user's reply:
   - **Insists on a draft** → re-invoke with `generate: true` and re-read the report. Log the file
     under **Done** and "draft — review every path" under **Needs review** only when `status` is
     `complete`, `generated` is `true`, and `fileGenerated` is a non-null path. Otherwise do not
     record success. Log the actual outcome (no candidates, existing policy preserved, or failure).
   - **Confirms no public endpoints** → log under **Skipped** ("internal-only service — no
     public-gateway AuthorizationPolicy needed") only after effective Gateway permissions have
     been checked (Step 1 attachment gate, including policies outside the chart). Remember
     `publicPolicyDecision: none-required` for Step 2.6. If that check has not happened, leave
     the decision unset and say so under **Needs review**.
   - **Declines / will write by hand (expected)** → log under **Needs review**: "Service owner to
     write the public-gateway AuthorizationPolicy by hand, candidates:" followed by the candidate
     paths copied into the log itself (not just a pointer to the report file).
7. `status: failed` → Error policy.

### Steps 2.6–2.8 — Validate (after any draft policy exists)

Invoke [`istio-migration-validate`](../istio-migration-validate/SKILL.md) with `chartPath`,
`repoRoot`, `interactive: false`, and `publicPolicyDecision` when Step 2.5 stored
`none-required` after the access check. It adds missing Istio guards (its one automatic fix), runs the
`helm template` checks for both mesh types, flags duplicate HTTPRoute rules, lints for leftover
no-backend / `RegularExpression` rules and policy presence, and flags imperative control-plane calls.

Copy `guardsAdded` (Done), `commandsRun`, `accessControlLint`, `controlPlaneCalls` and
`needsReview` into the log and rows 2.6 (guards, render), 2.7 (duplicates, lint), 2.8 (control-plane
calls). Do not log "no public-gateway AuthorizationPolicy found" again if Step 2.5 already logged it.
`status: failed` → Error policy.

---

## Final report

Mirror the migration guide checklist. Mark `[x]` when there is at least one **Done** entry and no
open **Needs review** entry for it; `[-] N/A (<reason>)` when legitimately not applicable (e.g.
no Maven plugin for Go, no sticky-session CRs); `[ ]` only means "action required".

```markdown
## Final report
- [ ] Existing mesh CRs converted to HTTPRoute CRs (incl. StatefulSession/LoadBalance → DestinationRule)
- [ ] Flagged features from Step 1 resolved
- [ ] Mesh-aware libraries replace old route-posting libraries
- [ ] SERVICE_MESH_TYPE wired in Helm values / Deployment (default Core)
- [ ] Maven plugin added and local build passes (Java only)
- [ ] HTTPRoute CRs generated from route registration code
- [ ] Public-gateway AuthorizationPolicy written by hand (or confirmed unnecessary)
- [ ] All Istio resources wrapped in the Istio conditional; none render under Core
- [ ] No duplicate HTTPRoute rules; access-control lint clean
- [ ] Imperative control-plane API calls flagged for review

Open items: <every remaining Needs review entry>
```

Close with a plain-language summary: what was applied automatically (Done count), what was skipped
and why, every remaining **Needs review** entry, and these local checks to run before pushing.
Require each `helm template` to exit 0 before reading its output. Do not pipe Helm into `grep`:
a failed Core render is not a pass.

```bash
istio_render=$(mktemp)
core_render=$(mktemp)
if helm template <chart> --set SERVICE_MESH_TYPE=Istio > "$istio_render"; then
  grep -E '^apiVersion: (gateway\.networking\.k8s\.io|networking\.istio\.io|security\.istio\.io|extensions\.istio\.io)/' "$istio_render"
else
  echo "FAIL: Istio render failed; do not treat empty grep output as a result"
fi
if helm template <chart> --set SERVICE_MESH_TYPE=Core > "$core_render"; then
  if grep -E '^apiVersion: (gateway\.networking\.k8s\.io|networking\.istio\.io|security\.istio\.io|extensions\.istio\.io)/' "$core_render"; then
    echo "FAIL: Istio resources rendered under Core"
  else
    echo "PASS: no Istio resources under Core"
  fi
else
  echo "FAIL: Core render failed; do not treat empty grep output as a pass"
fi
rm -f "$istio_render" "$core_render"
```

Tell the user to mention these effects in the merge description: the wider Gateway attachment,
regex cuts and omitted legacy 404 rules, and the requirement to commit generated routes.

---

## Operating rules

- Never skip the log; never invent values (versions, ports, names) or unresolved answers.
- Never run destructive or git-writing commands: no commit, push, tag, branch deletion, git config.
- Be explicit in chat: one-line summary plus the status row after each step.
- Reruns must not duplicate or corrupt output; sub-skills own the idempotency checks.
- Follow the Error policy on any unrecoverable failure.
- Never generate an `AuthorizationPolicy` without the user explicitly insisting (Step 2.5).

## Non-goals

Modifies only Helm templates, `values.yaml`, `values.schema.json`, `pom.xml`, `go.mod`, the consumer
`.gitignore` (the `.mesh-migration/` line) and files under `.mesh-migration/`. Does not raise PRs,
push, rewrite application logic or change git configuration.

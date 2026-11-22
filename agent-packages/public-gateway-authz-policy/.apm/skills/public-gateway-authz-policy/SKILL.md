---
name: public-gateway-authz-policy
description: >
  Recommend writing the public-gateway Istio AuthorizationPolicy by hand, and
  generate a draft only when the user explicitly insists. Covers the access
  control that replaces legacy allowed:false / deny:true / PUBLIC-PRIVATE-INTERNAL
  route visibility. Use when asked about public-gateway access control, an
  AuthorizationPolicy for a migrated service, or as a sub-skill of
  core-mesh-to-istio-migration.
---

# Public-gateway AuthorizationPolicy — recommend first, generate only on request

## Why this exists

Per [docs/istio/allowed-and-forbidden-routes.md](https://github.com/Netcracker/qubership-core-control-plane/blob/main/docs/istio/allowed-and-forbidden-routes.md):
every route is now registered on `public-gateway`, `private-gateway` and `internal-gateway-service`
regardless of its legacy PUBLIC/PRIVATE/INTERNAL type (see
[`core-mesh-crs-to-istio`](../core-mesh-crs-to-istio/SKILL.md) and
[`httproute-from-code`](../httproute-from-code/SKILL.md)). Cloud-Core ships a deny-unless-permit
`AuthorizationPolicy` for `public-gateway` (`action: ALLOW` with no `rules`), so **nothing of a
migrated service is reachable from the public gateway until the service brings its own `ALLOW`
policy**.

Only the service owner reliably knows which endpoints are meant to be public. **This skill's
default output is a recommendation to write that policy by hand** — a template, authoring tips,
and the candidate paths collected from the migration so far. It drafts a policy file **only**
when the caller explicitly asks for one anyway.

## Contract

### Inputs

| Input | Type | Required | Notes |
|---|---|---|---|
| `chartPath` | path | yes | Helm chart to read candidates from / write the draft into |
| `codePath` | path | no | Java source root to scan for `@Route` annotations (Step 2); defaults to `chartPath`'s sibling source tree when invoked by the orchestrator |
| `accessControl` | object | no | The `accessControl` report blocks from `core-mesh-crs-to-istio` and/or `httproute-from-code` (merged), used instead of re-reading report files when the orchestrator already has them |
| `routeLabels` | map | no | Labels for the draft resource, if generated; same meaning as in the sibling skills |
| `generate` | bool | no | `true` only when the user has explicitly insisted on a generated draft (see [Step 1](#step-1--always-recommend-manual-authoring-first)). Default `false` |
| `interactive` | bool | no | `true` only when a user invokes the skill directly; orchestrators and sub-agent wrappers pass `false` |
| `resolutions` | map `<unresolved id>: <answer>` | no | Answers to a previous run's `unresolved:` entries |

With `interactive: false`, never ask the user whether to generate — the caller (the orchestrator)
is responsible for having already asked and for setting `generate: true` only after the user
insisted. With `interactive: true` and `generate` unset, ask once (see Step 1) and set `generate`
from the answer.

### Outputs

In addition to the chat summary, write a machine-readable report to
`.mesh-migration/reports/public-gateway-authz-policy.yaml` (create the directory, and ensure
`.mesh-migration/` is listed in the repo's `.gitignore` — reports are working files, never
committed; the orchestrator handles both in orchestrated runs):

```yaml
reportSchema: 1
skill: public-gateway-authz-policy
status: complete            # complete | partial | failed
generatedAt: <ISO-8601>
recommendation: manual       # always "manual" — this skill never changes its own advice
candidates:
  legacyPublic: [<path>, ...]
  forbidden:
    - path: <path>
      methods: [<HTTP methods>]   # omitted when the original had no :method matcher
  deny:
    - path: <path>
      methods: [<HTTP methods>]   # omitted when the original had no :method matcher
  javaAnnotated: [<path>, ...]    # from Step 2, best-effort
generated: false             # true only if a draft file was written this run
fileGenerated: <path or null>
existingPolicy: <path or null>   # a hand-written public-gateway AuthorizationPolicy found in chartPath, if any
unresolved: []                # blocking user decisions, each {id, question, options, default}
needsReview:
  - <one line per ambiguous candidate or skipped item>
```

Consumers must ignore unknown report fields. A consumer that sees a `reportSchema` newer than its
own documentation must stop and report a contract mismatch instead of guessing field meanings.

### Side effects

- With `generate: false` (the default): **no file is written** other than the report.
- With `generate: true`: writes **one new file**,
  `<chartPath>/templates/public-gateway-authorization-policy.yaml` — see
  [Step 4](#step-4--generate-a-draft-only-when-generate-true). Never overwrites an existing
  hand-written policy that targets `public-gateway` (see [Step 3](#step-3--never-overwrite-an-existing-policy)).

---

## Step 1 — Always recommend manual authoring first

Regardless of `generate`, always produce this recommendation in the chat output before anything
else:

> Cloud-Core's public-gateway `AuthorizationPolicy` is deny-unless-permit — nothing of this
> service is reachable from the public gateway until it ships its own `ALLOW` policy. **It is
> recommended that you write this policy by hand**, because only you know which of the paths
> below are genuinely meant to be public. A generated draft can get you started, but every path
> in it still needs your review.

Then print the hand-authoring template (adapted from
[`docs/istio/crs-example.yaml`](https://github.com/Netcracker/qubership-core-control-plane/blob/main/docs/istio/crs-example.yaml)):

```yaml
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: '{{ .Values.SERVICE_NAME }}-public-gateway-allow'
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: public-gateway
  action: ALLOW
  rules:
    - to:
        - operation:
            paths:
              - /api/v1/my-service/public/*
              - /api/v1/my-service/health
```

Authoring tips to include:

- Paths are matched against the request path **as received by the gateway** — the route's `from`
  path, before any `URLRewrite`/`ReplacePrefixMatch`.
- For "this exact path, plus everything under it", list both `/x` and `/x/*` — Istio's path match
  here is not an implicit prefix.
- A templated path segment (`{id}`, `{**}` in Gateway API terms) becomes Istio's `{*}` (one
  segment) or `{**}` (multiple segments) in the `AuthorizationPolicy` path syntax.
- To keep a sub-path excluded while allowing the rest of a broader path, add it to that rule's
  `to[].operation.notPaths` rather than writing a second, narrower `ALLOW` rule — narrower `ALLOW`
  rules do not subtract from broader ones, they only add.
- Only add `methods` / `notMethods` when the legacy route actually restricted by HTTP method (a
  `:method` header matcher on a forbidden or deny rule). Most routes do not need it — leaving
  `methods` unset allows every method.

### Ask once, only with `interactive: true` and `generate` unset

> I can draft a starting-point `AuthorizationPolicy` from the candidates below, but every path in
> it still needs your review before merging. Do you want me to generate the draft file anyway?

Set `generate` from the answer. Never ask a second time in the same run.

---

## Step 2 — Collect candidates

Gather candidate paths without generating anything yet — this runs regardless of `generate`,
because the recommendation in Step 1 is more useful with real paths attached.

1. **From sibling reports.** If `accessControl` was not passed directly as an input, read
   `.mesh-migration/reports/core-mesh-crs-to-istio.yaml` and
   `.mesh-migration/reports/httproute-from-code.yaml` (whichever exist) and merge their
   `accessControl` blocks:
   - `legacyPublic[]` → `candidates.legacyPublic`.
   - `forbidden[]` / `deny[]` → `candidates.forbidden` / `candidates.deny`, each carrying
     `methods` only when the source entry had it.
   If neither report exists, continue — this is not an error, just fewer candidates.
2. **From Java annotations (best effort).** Scan `<codePath>` (`*.java`, same ignore list as
   [`httproute-from-code`](../httproute-from-code/SKILL.md) Step 1: `target/`, `src/test/`, etc.)
   for:
   - `@Route(type = RouteType.PUBLIC)` / `@Route(value = RouteType.PUBLIC)` — the default `@Route`
     type when neither `type`/`value` nor `gateways` is set is `RouteType.INTERNAL`, **not**
     `PUBLIC`; do not treat an unannotated or bare `@Route` as public.
   - `@Route(gateways = "public-gateway-service")` (array or single-value form).
   Resolve each hit's path by combining the class-level `@Path`/`@GatewayRequestMapping` (if any)
   with the method-level path, best effort — do not fail the run if resolution is ambiguous; add
   the class+method name to `needsReview:` instead and skip that one candidate.
   Add resolved paths to `candidates.javaAnnotated`.
3. Deduplicate within each list. Do not merge `javaAnnotated` into `legacyPublic` — they come from
   different sources and may disagree; report them separately so a human can reconcile.

Print all four candidate lists in the chat summary even when `generate` is `false` — this is the
actionable output of a recommendation-only run.

---

## Step 3 — Never overwrite an existing policy

Before generating anything, search `<chartPath>/templates/` (and any `-istio.yaml` siblings) for
an existing `AuthorizationPolicy` whose `spec.targetRefs` includes Gateway `public-gateway`.

- **Found** → record its path under `existingPolicy:` in the report, do **not** write a draft
  file even if `generate: true`, and add a `needsReview:` entry: "A public-gateway
  AuthorizationPolicy already exists at `<path>` — this skill does not merge into or overwrite a
  hand-written policy. Review it against the candidates above yourself."
- **Not found** → proceed to [Step 4](#step-4--generate-a-draft-only-when-generate-true) if
  `generate: true`, otherwise finish with `generated: false`.

---

## Step 4 — Generate a draft (only when `generate: true`)

Build **exactly one** `AuthorizationPolicy`, `action: ALLOW` only — never generate a `DENY`
policy; legacy `deny:true` / `allowed:false` paths are excluded via `notPaths` on the `ALLOW`
rule instead (see below), the same effect with one policy to review instead of two.

1. **Base rule paths** — `to[0].operation.paths`: one entry per distinct path in
   `candidates.legacyPublic` (and, if the user told you to include them, `candidates.javaAnnotated`
   — do not include `javaAnnotated` silently, since it was never actually reachable from the
   public gateway through a declarative/code route; ask or flag instead of guessing). Render each
   path as given; render a templated segment as `{*}` (single segment) — Gateway API's `{id}` /
   Istio's `{*}` are both single-segment, so no `{**}` inference is made here. For a path meant to
   cover everything under it, add the `/x/*` sibling explicitly — do not assume a trailing
   wildcard.
2. **`notPaths`** — for every `forbidden[]` / `deny[]` candidate that has **no** `methods` and
   falls under a base-rule path, add it to that rule's `to[0].operation.notPaths`.
3. **Method-restricted candidates** — for every `forbidden[]` / `deny[]` candidate that **does**
   carry `methods`:
   - add its path to the base rule's `notPaths` too (fully excluding it from the broad rule);
   - add a **second** rule that allows everything on that exact path except the restricted
     methods:
     ```yaml
     - to:
         - operation:
             paths:
               - <path>
             notMethods: [<methods>]
     ```
   This reproduces the legacy behavior (that method was 404 on this path, every other method was
   allowed) with Gateway API's `ALLOW`-only, no-negated-rule-count model.
4. **Name and labels.** Resource name `{{ .Values.SERVICE_NAME }}-public-gateway-allow`. Apply
   `routeLabels` verbatim if provided; otherwise omit `metadata.labels`.
5. **Wrap and header.** Wrap the whole document in the Istio guard, and prepend:
   ```yaml
   # -----------------------------------------------------------------------------
   # DRAFT — generated on explicit request. Every path below needs human review
   # before merging: confirm it matches what should really be public, and delete
   # any candidate that should not be.
   # -----------------------------------------------------------------------------
   ```
6. Write to `<chartPath>/templates/public-gateway-authorization-policy.yaml`. Record the path
   under `fileGenerated:` and set `generated: true`.
7. Always add a `needsReview:` entry: "Generated public-gateway AuthorizationPolicy is a draft —
   verify every path before merging."

### Full example

Input candidates:
```yaml
legacyPublic: [/api/v1/billing/invoices, /api/v1/billing/invoices/*]
forbidden:
  - path: /api/v1/billing/admin
  - path: /api/v1/billing/invoices/bulk
    methods: [DELETE]
```

Generated draft:
```yaml
{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}
# -----------------------------------------------------------------------------
# DRAFT — generated on explicit request. Every path below needs human review
# before merging: confirm it matches what should really be public, and delete
# any candidate that should not be.
# -----------------------------------------------------------------------------
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: '{{ .Values.SERVICE_NAME }}-public-gateway-allow'
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: public-gateway
  action: ALLOW
  rules:
    - to:
        - operation:
            paths:
              - /api/v1/billing/invoices
              - /api/v1/billing/invoices/*
            notPaths:
              - /api/v1/billing/admin
              - /api/v1/billing/invoices/bulk
    - to:
        - operation:
            paths:
              - /api/v1/billing/invoices/bulk
            notMethods: [DELETE]
{{- end }}
```

`/api/v1/billing/admin` is not itself a base-rule path, but `notPaths` on an `ALLOW` rule only
narrows that rule's own match — it does not need to be a prefix of a listed `paths` entry to have
an effect here, since without it the `/api/v1/billing/invoices/*` wildcard would not reach it
anyway. It is included defensively in case a future edit widens the base paths.

---

## Non-goals

- Never generate a `DENY` `AuthorizationPolicy`.
- Never add `methods` / `notMethods` to a rule unless a `forbidden`/`deny` candidate explicitly
  carried method information.
- Never overwrite or merge into an existing public-gateway `AuthorizationPolicy` (see Step 3).
- Never generate anything when `generate` is not explicitly `true` — recommending manual authoring
  is the default, complete behavior of a run, not a fallback.
- Does not modify `HTTPRoute`, `Gateway`, or any other resource — read-only there, writes only the
  one new draft file (if any) and the report.

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

Per [docs/istio/allowed-and-forbidden-routes.md](https://github.com/Netcracker/qubership-core-control-plane/blob/main/docs/istio/allowed-and-forbidden-routes.md),
every route is now registered on `public-gateway`, `private-gateway` and `internal-gateway-service`
regardless of its legacy PUBLIC/PRIVATE/INTERNAL type. Cloud-Core ships a deny-unless-permit
`AuthorizationPolicy` for `public-gateway` (`action: ALLOW`, no `rules`), so **nothing of a service
is reachable from the public gateway until the service brings its own `ALLOW` policy**.

Two consequences, to state to the user in plain words:

- A service that is **not** meant to be called from outside the cluster (internal/private only)
  needs **no** policy at all — Cloud-Core already blocks it on the public gateway.
- Only the service owner knows which endpoints are really public. **Write the policy by hand;**
  this skill drafts one only when explicitly told to.

## Contract

### Inputs

| Input | Type | Required | Notes |
|---|---|---|---|
| `chartPath` | path | yes | Helm chart to search for an existing policy / write the draft into |
| `codePath` | path | no | Java source root to scan (Step 2) |
| `accessControl` | object | no | Merged `accessControl` blocks of `core-mesh-crs-to-istio` and `httproute-from-code`; if absent, read their report files |
| `routeLabels` | map | no | Labels for the draft resource |
| `generate` | bool | no | `true` only after the user explicitly insisted on a draft. Default `false` |
| `interactive` | bool | no | `true` only when a user invokes the skill directly; orchestrators pass `false` |

With `interactive: false` never ask the user; the caller asks and sets `generate`. With
`interactive: true` and `generate` unset, ask once in Step 3.

### Outputs

Write `.mesh-migration/reports/public-gateway-authz-policy.yaml` (create the directory; ensure
`.mesh-migration/` is in the repo `.gitignore` — the orchestrator does both when orchestrated):

```yaml
reportSchema: 1
skill: public-gateway-authz-policy
status: complete            # complete | failed
generatedAt: <ISO-8601>
recommendation: manual      # always "manual"
candidates:
  # matchType optional (missing = prefix); methods optional; gateways only from core-mesh-crs-to-istio
  legacyPublic: [{path: <path>, matchType: <prefix|exact|reduced-regexp>}]
  forbidden: [{path: <path>, methods: [..], matchType: <prefix|exact|reduced-regexp>, gateways: [..]}]
  deny:      [{path: <path>, methods: [..], matchType: <...>, gateways: [..]}]
  javaAnnotated: [<path>, ...]
generated: false            # true only if a draft file was written this run
fileGenerated: <path or null>
existingPolicy: <path or null>
unresolved: []
needsReview:
  - <one line per item>
```

Consumers ignore unknown fields. A consumer seeing a newer `reportSchema` than it knows must stop
and report a contract mismatch. On an unrecoverable error set `status: failed` and explain under
`needsReview:`.

### Side effects

With `generate: false`: nothing except the report. With `generate: true`: at most one new file,
`<chartPath>/templates/public-gateway-authorization-policy.yaml`. Step 4 writes no file when a
sibling report is stale, or when there is neither a base path nor a public-gateway deny entry.

---

## Step 1 — Check for an existing policy

Search `<chartPath>/templates/` for an `AuthorizationPolicy` whose `spec.targetRefs` includes Gateway
`public-gateway`. If found: set `existingPolicy:`, **never write a draft** (even with
`generate: true`), add `needsReview:` "Existing public-gateway AuthorizationPolicy at `<path>` — review
it against the candidates; this skill never merges into or overwrites it." and still continue with
Step 2 so the candidates are reported.

## Step 2 — Collect candidates

1. From `accessControl` (input, else the two sibling report files; missing files are not an error):
   `legacyPublic[]`, `forbidden[]`, `deny[]`. Copy each entry's `from` to `path`, and keep
   `methods`, `matchType`, and `gateways` when present.
   A sibling report file that exists but has no `accessControl` key was written by a version that
   omitted the block (`core-mesh-crs-to-istio` before 3.1.0, `httproute-from-code` before 2.2.0),
   so an empty inventory cannot be told apart from one that was never collected. Do not read
   candidates from it. Add `needsReview:`
   "`<report file>` predates access-control collection — rerun `<skill>` before relying on these
   candidates." and treat the inventory as stale in Step 4.
2. Best-effort Java scan of `<codePath>/**/*.java` (skip `target/`, `src/test/`) for routes that were
   public: `@Route(type = RouteType.PUBLIC)`, `@Route(value = RouteType.PUBLIC)`,
   `@Route(gateways = "public-gateway-service")`. A bare `@Route` is `INTERNAL`, **not** public.
   Build the full path from the class-level mapping (`@Path`, `@GatewayRequestMapping`,
   `@RequestMapping`) plus the method-level mapping (`@Path`, `@GetMapping`, `@PostMapping`,
   `@PutMapping`, `@DeleteMapping`, `@PatchMapping`, `@RequestMapping`). If a path cannot be resolved,
   add the class+method to `needsReview:` and skip it.
3. Deduplicate each list. Keep `javaAnnotated` separate from `legacyPublic`.

## Step 3 — Print the recommendation (always)

Print this, then all four candidate lists (even when nothing is generated):

> Cloud-Core's public-gateway policy is deny-unless-permit — nothing of this service is reachable
> from the public gateway until it ships its own `ALLOW` policy. I recommend you write it by hand:
> only you know which paths must be public. If this service has no public endpoints, you do not
> need any policy.

Hand-authoring template:

```yaml
{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}
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
              - /api/v1/my-service/public
              - /api/v1/my-service/public/*
{{- end }}
```

Tips:

- Match the path **as received by the gateway** (the route's `from`, before any rewrite).
- Istio path match is exact unless it ends with `*`: to cover `/x` and everything under it, list
  both `/x` and `/x/*`.
- `ALLOW` policies only add; to exclude a path another policy allows, use a separate
  `action: DENY` policy (DENY is evaluated first). Set `methods` only to restrict by HTTP method.
- Regex is not supported in `paths`; use exact paths or `/x/*` prefixes. Replace templated path
  segments (`{id}`) with Istio's `{*}` wildcard (otherwise `{id}` is matched literally).
- See [docs/istio/crs-example.yaml](https://github.com/Netcracker/qubership-core-control-plane/blob/main/docs/istio/crs-example.yaml).

With `interactive: true` and `generate` unset (and no `existingPolicy`), ask once:
"Do you want me to generate a draft anyway? Every path in it still needs your review."

## Step 4 — Generate a draft (only when `generate: true` and `existingPolicy` is null)

If Step 2 found a stale report, write no file: the candidate lists may be missing routes. Record
`needsReview:` "No draft written — rerun the converters listed above, then rerun this skill." and
stop.

**Base paths** = `candidates.legacyPublic` plus `candidates.javaAnnotated` (tell the user both were
included). A `javaAnnotated` path is a `prefix` (code routes always render as `PathPrefix`). Replace
any `{var}` path segment with `{*}`.

**Expand by match type.** Every path written to `paths` or `notPaths` below is expanded from its
entry's `matchType`:

- `exact` → `p` only. Never add `p/*`: that would open every descendant of an exact public route.
- `prefix` → `p` and `p/*`.
- `reduced-regexp` → `p` and `p/*` (item 4 below adds the review entry).
- no `matchType` → treat as `prefix`.

An entry **lies under** a base path `b` when its path equals `b`, or, only if `b` is a `prefix` or
`reduced-regexp` entry, when its path starts with `<b>/`.

**Public deny entries** = the `deny[]` entries whose `gateways` includes `public-gateway-service`
(see item 3).

- If **base paths** are empty, **do not emit an `ALLOW` policy** (an `ALLOW` policy with only
  `notPaths` would allow every path on `public-gateway`). Still emit the `DENY` policy below if
  there are public deny entries. If there are none either, write no file, keep `generated: false`
  and `fileGenerated: null`, record the item 3 `needsReview:` entries for any other `deny[]`
  entries plus "No public candidates found — write a policy by hand, or skip it if the service has
  no public endpoints.", and stop.

Write one file wrapped in `{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}` … `{{- end }}`, starting
with a comment `# DRAFT — generated on request; review every path before merging.`:

1. **ALLOW policy** `{{ .Values.SERVICE_NAME }}-public-gateway-allow` (only when base paths are
   non-empty): one rule, `paths` = every base path, expanded by match type.
2. **Forbidden candidates** (`forbidden[]`) that lie under a base path; ignore the rest (already
   denied by default). Add each one, expanded, to the first rule's `notPaths`. This fails closed:
   an entry that has `gateways` without `public-gateway-service` was forbidden only on another
   gateway, so also add `needsReview:` "Forbidden route `<path>` applied on `<gateways>`; the draft
   blocks it on public-gateway too — remove it from `notPaths` if it must stay public."
   Then, for an entry with `methods`, add a method exception
   `to.operation: {paths: [<exception paths>], notMethods: [<methods>]}` as a separate rule, where
   the exception paths are the entry's expanded paths limited to what its base path opens: under an
   `exact` base that is `p` only, never `p/*`. Skip the exception **if the entry overlaps another
   forbidden entry**, with or without `methods`. Two entries overlap when one lies under the other
   (the same test as for base paths). ALLOW rules are OR-ed, so an exception rule for one entry
   would allow requests that the overlapping entry blocks. For every overlapping pair, emit no
   exception rule for either entry (both stay fully excluded through `notPaths`) and add
   `needsReview:` "Forbidden entries `<path A>` and `<path B>` overlap — the draft blocks both on
   every method; restore the allowed methods by hand."
3. **DENY policy** `{{ .Values.SERVICE_NAME }}-public-gateway-deny`: one rule per `deny[]` entry
   whose `gateways` includes `public-gateway-service`, with `paths` expanded by match type, plus
   `methods: [...]` when the entry has them. Required because `deny: true` overrode paths allowed
   by *other* policies, which only DENY can do. Do not draft any other `deny[]` entry on
   `public-gateway`: a public DENY for a path denied on another gateway can block a public route
   that a different policy allows. Report it instead, with `needsReview:` "Deny route `<path>`
   applied on `<gateways>`, not on public-gateway — not drafted." An entry with no `gateways`
   (written by an older converter) gets the same treatment, with `<gateways>` given as `unknown`.
   Emit the DENY policy only when at least one entry qualifies.
4. **Regex-derived entries** (`matchType: reduced-regexp`) use their reduced prefix; add a
   `needsReview:` entry for each. Never write a regex into `paths`/`notPaths`.
5. Apply `routeLabels` to `metadata.labels` if provided; otherwise omit labels.
6. Record `fileGenerated:`, `generated: true`, and `needsReview:` "Generated policy is a draft —
   verify every path."

Example — candidates `legacyPublic: [{path: /api/v1/billing/invoices, matchType: prefix}]`,
`forbidden: [{path: /api/v1/billing/invoices/bulk, matchType: prefix, methods: [DELETE]}]`,
`deny: [{path: /api/v1/billing/invoices/internal, matchType: prefix, gateways: [public-gateway-service]}]`:

```yaml
rules:   # ALLOW policy
  - to:
      - operation:
          paths: [/api/v1/billing/invoices, /api/v1/billing/invoices/*]
          notPaths: [/api/v1/billing/invoices/bulk, /api/v1/billing/invoices/bulk/*]
  - to:
      - operation:
          paths: [/api/v1/billing/invoices/bulk, /api/v1/billing/invoices/bulk/*]
          notMethods: [DELETE]
---
# DENY policy: action: DENY
rules:
  - to:
      - operation:
          paths: [/api/v1/billing/invoices/internal, /api/v1/billing/invoices/internal/*]
```

Example — an exact public route with a private descendant. Candidates
`legacyPublic: [{path: /api/v1/billing/status, matchType: exact}]`; the private route
`/api/v1/billing/status/details` is not a candidate. The ALLOW rule opens the exact path only, so
the descendant stays blocked by Cloud-Core's default policy:

```yaml
rules:   # ALLOW policy
  - to:
      - operation:
          paths: [/api/v1/billing/status]
```

Example — overlapping method-restricted entries. Candidates
`legacyPublic: [{path: /api/v1/orders, matchType: prefix}]`,
`forbidden: [{path: /api/v1/orders/admin, matchType: prefix, methods: [DELETE]},
{path: /api/v1/orders/admin/purge, matchType: prefix, methods: [POST]}]`. Separate exceptions would
let `POST /api/v1/orders/admin/purge` through the first entry's `notMethods: [DELETE]` rule. The
entries overlap, so neither gets an exception, and `needsReview:` names the pair:

```yaml
rules:   # ALLOW policy
  - to:
      - operation:
          paths: [/api/v1/orders, /api/v1/orders/*]
          notPaths: [/api/v1/orders/admin, /api/v1/orders/admin/*,
                     /api/v1/orders/admin/purge, /api/v1/orders/admin/purge/*]
```

Example — a deny entry from another gateway. Candidates
`deny: [{path: /api/v1/billing/ledger/export, matchType: prefix, gateways: [private-gateway-service]}]`
and nothing else. There is no ALLOW policy (no base paths) and no DENY policy (no entry on
`public-gateway-service`), so no file is written. `needsReview:` reports the deny route against
`private-gateway-service`.

Example — overlap with an entry that has no methods. Candidates
`legacyPublic: [{path: /api/v1/orders, matchType: prefix}]`,
`forbidden: [{path: /api/v1/orders/admin, matchType: prefix},
{path: /api/v1/orders/admin/report, matchType: prefix, methods: [DELETE]}]`. An exception rule for
the second entry would reopen `GET /api/v1/orders/admin/report`, which the first entry blocks on
every method. Both paths go to `notPaths`, no exception rule is written, and `needsReview:` names
the pair.

Example — a deny entry with no `gateways` (from a converter before 3.1.0). Candidates
`legacyPublic: [{path: /api/v1/billing/invoices, matchType: prefix}]`,
`deny: [{path: /api/v1/billing/invoices/internal, matchType: prefix}]`. The ALLOW policy is written
as usual, the DENY policy is not, and `needsReview:` reports the deny route against `unknown`.

## Non-goals

- Never overwrite or merge into an existing public-gateway policy.
- Never generate anything when `generate` is not explicitly `true`.
- Never modify `HTTPRoute`, `Gateway` or any other resource.

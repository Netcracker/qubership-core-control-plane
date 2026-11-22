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
  legacyPublic: [{path: <path>, matchType: <prefix|exact|reduced-regexp>}]               # matchType optional
  forbidden: [{path: <path>, methods: [..], matchType: <prefix|exact|reduced-regexp>}]   # methods optional
  deny:      [{path: <path>, methods: [..], matchType: <...>}]
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

With `generate: false`: nothing except the report. With `generate: true`: one new file,
`<chartPath>/templates/public-gateway-authorization-policy.yaml`.

---

## Step 1 — Check for an existing policy

Search `<chartPath>/templates/` for an `AuthorizationPolicy` whose `spec.targetRefs` includes Gateway
`public-gateway`. If found: set `existingPolicy:`, **never write a draft** (even with
`generate: true`), add `needsReview:` "Existing public-gateway AuthorizationPolicy at `<path>` — review
it against the candidates; this skill never merges into or overwrites it." and still continue with
Step 2 so the candidates are reported.

## Step 2 — Collect candidates

1. From `accessControl` (input, else the two sibling report files; missing files are not an error):
   `legacyPublic[]`, `forbidden[]`, `deny[]` (keep `methods` and `matchType` when present).
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

**Base paths** = `candidates.legacyPublic` plus `candidates.javaAnnotated` (tell the user both were
included). Replace any `{var}` path segment with `{*}`.

- If **base paths** are empty, **do not emit an `ALLOW` policy** (an `ALLOW` policy with only
  `notPaths` would allow every path on `public-gateway`). Still emit the `DENY` policy below if
  `deny[]` is non-empty; if both are empty, write no file, record `needsReview:` "No public
  candidates found — write a policy by hand, or skip it if the service has no public endpoints."
  and stop.

Write one file wrapped in `{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}` … `{{- end }}`, starting
with a comment `# DRAFT — generated on request; review every path before merging.`:

1. **ALLOW policy** `{{ .Values.SERVICE_NAME }}-public-gateway-allow` (only when base paths are
   non-empty): one rule, `paths` = every base path `p` as `p` and `p/*`.
2. **Forbidden candidates** (`forbidden[]`) that lie under a base path (equal to it or starting with
   `<base>/`); ignore the rest (already denied by default):
   - no `methods` → add `p` and `p/*` to the rule's `notPaths`;
   - with `methods` → add `p` and `p/*` to `notPaths` **and** add a second rule
     `to.operation: {paths: [p, p/*], notMethods: [<methods>]}`.
3. **DENY policy** `{{ .Values.SERVICE_NAME }}-public-gateway-deny`, only if `deny[]` is non-empty:
   one rule per entry, `paths: [p, p/*]`, plus `methods: [...]` when the entry has them. Required
   because `deny: true` overrode paths allowed by *other* policies, which only DENY can do.
4. **Regex-derived entries** (`matchType: reduced-regexp`) use their reduced prefix; add a
   `needsReview:` entry for each. Never write a regex into `paths`/`notPaths`.
5. Apply `routeLabels` to `metadata.labels` if provided; otherwise omit labels.
6. Record `fileGenerated:`, `generated: true`, and `needsReview:` "Generated policy is a draft —
   verify every path."

Example — candidates `legacyPublic: [/api/v1/billing/invoices]`,
`forbidden: [{path: /api/v1/billing/invoices/bulk, methods: [DELETE]}]`,
`deny: [{path: /api/v1/billing/invoices/internal}]`:

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

## Non-goals

- Never overwrite or merge into an existing public-gateway policy.
- Never generate anything when `generate` is not explicitly `true`.
- Never modify `HTTPRoute`, `Gateway` or any other resource.

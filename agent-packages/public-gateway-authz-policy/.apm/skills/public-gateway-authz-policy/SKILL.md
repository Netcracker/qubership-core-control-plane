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
routes that were already public are registered on `public-gateway`, `private-gateway` and
`internal-gateway-service`. Formerly non-public routes stay off `public-gateway` until effective
access has been checked. Cloud-Core ships a deny-unless-permit `AuthorizationPolicy` for
`public-gateway` (`action: ALLOW`, no `rules`). That policy does not, by itself, prove a route is
unreachable: a project-level ALLOW that targets the same Gateway can match it. See section 3a of
[docs/istio/crs-example.yaml](https://github.com/Netcracker/qubership-core-control-plane/blob/main/docs/istio/crs-example.yaml).

Two consequences, to state to the user in plain words:

- A service that is **not** meant to be called from outside the cluster needs **no** policy of its
  own only after effective public-gateway access has been checked and no ALLOW matches its paths.
- Only the service owner knows which endpoints are really public. **Write the policy by hand.**
  This skill drafts one only when explicitly told to. Do not forbid PRIVATE, INTERNAL, or
  `allowed: false` routes on the private gateway. A legacy deny may also matter on the gateway it
  came from; report that gateway, and draft a public DENY only when the entry was on the public
  gateway.

## Contract

### Inputs

| Input | Type | Required | Notes |
|---|---|---|---|
| `chartPath` | path | yes | Helm chart to search for an existing policy / write the draft into |
| `codePath` | path | no | Java source root to scan (Step 2) |
| `accessControl` | object | no | Merged producer `accessControl` blocks. Used only when `collected: true` and `chartPath` matches this chart |
| `routeLabels` | map | no | Labels for the draft resource |
| `generate` | bool | no | `true` only after the user explicitly insisted on a draft. Default `false` |
| `interactive` | bool | no | `true` only when a user invokes the skill directly; orchestrators pass `false` |

With `interactive: false` never ask the user; the caller asks and sets `generate`. With
`interactive: true` and `generate` unset, ask once in Step 3.

### Outputs

Write `.mesh-migration/reports/public-gateway-authz-policy.yaml` (create the directory; ensure
`.mesh-migration/` is in the repo `.gitignore` — the orchestrator does both when orchestrated;
a standalone run does both itself):

```yaml
reportSchema: 1
skill: public-gateway-authz-policy
chartPath: <chartPath>
status: complete            # complete | failed
generatedAt: <ISO-8601>
recommendation: manual      # always "manual"
inventory: accepted | rejected
candidates:
  legacyPublic: [{path: <path>, matchType: <prefix|reduced-regexp>, gateways: [...]}]
  forbidden: [{path: <path>, methods: [..], matchType: <prefix|reduced-regexp>, gateways: [...]}]
  deny:      [{path: <path>, methods: [..], matchType: <...>, gateways: [...]}]
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

`generate: false` still writes the report. A standalone run may also append `.mesh-migration/` to
the repo `.gitignore` when that line is missing. Neither of those creates policy YAML. Policy YAML
is created only when `generate: true` and Step 4 actually writes
`<chartPath>/templates/public-gateway-authorization-policy.yaml`. An orchestrated run leaves
`.gitignore` to the orchestrator.

---

## Step 1 — Check for an existing policy

Search `<chartPath>/templates/` for an `AuthorizationPolicy` whose `spec.targetRefs` includes Gateway
`public-gateway`. If found: set `existingPolicy:`, **never write a draft** (even with
`generate: true`), add `needsReview:` "Existing public-gateway AuthorizationPolicy at `<path>` — review
it against the candidates; this skill never merges into or overwrites it." and still continue with
Step 2 so the candidates are reported.

## Step 2 — Collect candidates

Tie every reused inventory to this chart before using it.

1. Producer inventory, in this order:
   - If `accessControl` was passed: accept it only when `collected: true` and its `chartPath` is
     absent or equal to the selected chart. A missing `collected` flag is an older inventory, not
     an empty one — including a schema-1 report that never had `accessControl`.
   - Otherwise read `.mesh-migration/reports/core-mesh-crs-to-istio.yaml` and
     `httproute-from-code.yaml`. Accept a file only when its top-level `chartPath` equals the
     selected chart and `accessControl.collected` is `true`.
   - If the input or a file fails those checks, set `inventory: rejected` and `status: failed`.
     `needsReview:` "Reports cannot be tied to `<chartPath>`. Re-run core-mesh-crs-to-istio and
     httproute-from-code on this chart, or pass accessControl with collected: true. Do not treat
     this as an empty inventory." Do not recommend or draft from the rejected lists.
   - When both producers have `collected: true` and empty lists, that is a finished empty
     inventory. Set `inventory: accepted` and continue.
2. From an accepted inventory, copy `legacyPublic[]`, `forbidden[]`, and `deny[]`. Keep `methods`,
   `matchType`, and `gateways`.
3. Best-effort Java scan of `<codePath>/**/*.java` (skip `target/`, `src/test/`) for routes that were
   public: `@Route(type = RouteType.PUBLIC)`, `@Route(value = RouteType.PUBLIC)`,
   `@Route(gateways = "public-gateway-service")`. A bare `@Route` is `INTERNAL`, **not** public.
   Build the full path from the class-level mapping (`@Path`, `@GatewayRequestMapping`,
   `@RequestMapping`) plus the method-level mapping (`@Path`, `@GetMapping`, `@PostMapping`,
   `@PutMapping`, `@DeleteMapping`, `@PatchMapping`, `@RequestMapping`). If a path cannot be resolved,
   add the class+method to `needsReview:` and skip it.
4. Deduplicate each list. Keep `javaAnnotated` separate from `legacyPublic`.

## Step 3 — Print the recommendation (always, when inventory is accepted)

Print this, then all four candidate lists (even when nothing is generated):

> I recommend you write the public-gateway AuthorizationPolicy by hand: only you know which paths
> must be public. Cloud-Core's deny-unless-permit policy is not the only ALLOW that can target
> this Gateway. Check project-level policies before you decide the service needs no policy. If it
> truly has no public endpoints, and no other ALLOW matches its paths, you do not need a policy.

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
- Regex is not supported in `paths`. Use the reduced prefix the producer recorded. Do not write a
  regex, and do not ask for a manual review of a non-conflicting cut.
- Do not target `private-gateway` or `internal-gateway-service`. Restrictions from `allowed: false`
  and from PRIVATE / INTERNAL annotations are public-gateway concerns only.
- See [docs/istio/crs-example.yaml](https://github.com/Netcracker/qubership-core-control-plane/blob/main/docs/istio/crs-example.yaml).

With `interactive: true` and `generate` unset (and no `existingPolicy`), ask once:
"Do you want me to generate a draft anyway? Every path in it still needs your review."

## Step 4 — Generate a draft (only when `generate: true`, inventory is accepted, and `existingPolicy` is null)

**Base paths** = `candidates.legacyPublic` plus `candidates.javaAnnotated` (tell the user both were
included). Replace any `{var}` path segment with `{*}`. Do not special-case an exact match. Emit
`p` and `p/*` for every base path.

- If **base paths** are empty, **do not emit an `ALLOW` policy** (an `ALLOW` policy with only
  `notPaths` would allow every path on `public-gateway`). Still emit the public `DENY` policy
  below when a confirmed public deny exists. If nothing is emitted, write no file, set
  `generated: false` and `fileGenerated: null`, record `needsReview:` "No public candidates found —
  write a policy by hand, or skip it if the service has no public endpoints." and stop.
- Set `generated: true` and `fileGenerated:` only after the file has been written. A failed write
  or an empty draft leaves both as `false` / `null` and sets `status: failed`.

Write one file wrapped in `{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}` … `{{- end }}`, starting
with a comment `# DRAFT — generated on request; review every path before merging.`:

1. **ALLOW policy** `{{ .Values.SERVICE_NAME }}-public-gateway-allow` (only when base paths are
   non-empty): one rule, `paths` = every base path `p` as `p` and `p/*`.
2. **Forbidden candidates** (`forbidden[]`) that lie under a base path (equal to it or a whole-segment
   prefix). Ignore the rest (already denied by default). These exclusions apply only to the public
   ALLOW. Do not emit a private-gateway policy for them.
   - Build one ALLOW exception per forbidden path. `notMethods` is the union of `methods` on that
     entry and on every forbidden ancestor that covers it. Descendants do not add methods to an
     ancestor.
   - An entry with no `methods` forbids every method. If it or an ancestor has no `methods`, add
     `p` and `p/*` to the broad rule's `notPaths` and emit no exception for that path.
   - Otherwise add `p` and `p/*` to the broad rule's `notPaths`, and add one exception rule
     `to.operation: {paths: [p, p/*], notMethods: [<union>]}`. Carve a stricter descendant out of
     the ancestor exception with `notPaths`, and put the ancestor's methods on the descendant
     exception as well.
   - Two entries on the same path are expressible: one exception, `notMethods` = the union.
   - Leave an overlap the draft cannot express for manual review and emit no exception that would
     allow a method another overlapping entry forbids. The case is a full deny (no `methods`)
     overlapping a method-restricted entry: an exception would allow methods the full deny bars.
     `needsReview:` names both paths. Add a regression for this shape and for two method lists on
     one path; see [tests/README.md](tests/README.md).
3. **DENY policy** `{{ .Values.SERVICE_NAME }}-public-gateway-deny`, only for `deny[]` entries
   whose `gateways` include `public-gateway-service` (or that are otherwise confirmed to apply to
   `public-gateway`). One rule per such entry, `paths: [p, p/*]`, plus `methods` when the entry
   has them. Required because a public `deny: true` overrode paths allowed by *other* policies.
   Every other deny stays out of this file. Report it against its original gateway:
   "Deny `<path>` applies to `<gateways>` and was not drafted onto public-gateway." A missing
   `gateways` list is not confirmation: do not draft it; ask for a fresh inventory.
   Do not write a DENY that targets `private-gateway`.
4. **Regex-derived entries** (`matchType: reduced-regexp`) use their reduced prefix. Do not add
   `needsReview` unless the producer set `conflicts: true`. Never write a regex into
   `paths`/`notPaths`.
5. Apply `routeLabels` to `metadata.labels` if provided; otherwise omit labels.
6. Record `fileGenerated:`, `generated: true`, and `needsReview:` "Generated policy is a draft —
   verify every path." only when the file was written.

Example — candidates `legacyPublic: [/api/v1/billing/invoices]`,
`forbidden: [{path: /api/v1/billing/invoices/bulk, methods: [DELETE], gateways: [public-gateway-service]}]`,
`deny: [{path: /api/v1/billing/invoices/internal, gateways: [public-gateway-service]}]`
plus a second deny `{path: /api/v1/billing/invoices, gateways: [egress-gateway]}` which is reported
and not drafted:

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
# DENY policy: action: DENY — public-gateway-service entries only
rules:
  - to:
      - operation:
          paths: [/api/v1/billing/invoices/internal, /api/v1/billing/invoices/internal/*]
```

## Non-goals

- Never overwrite or merge into an existing public-gateway policy.
- Never generate anything when `generate` is not explicitly `true`.
- Never modify `HTTPRoute`, `Gateway` or any other resource.
- Never target `private-gateway` from this skill.

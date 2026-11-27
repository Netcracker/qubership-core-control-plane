# core-mesh-crs-to-istio — E2E Tests

## Stickiness / load balancing

```text
Run skill `core-mesh-crs-to-istio` on file
`agent-packages/core-mesh-crs-to-istio/.apm/skills/core-mesh-crs-to-istio/tests/input.yaml`.

Compare the result with `tests/expected-output.yaml` and report any differences.
```

| # | CR | Input condition | Expected output |
|---|----|-----------------|-----------------|
| 1 | `StatefulSession` | cookie, ttl=0 | DestinationRule with `httpCookie`, ttl `"0s"` |
| 2 | `StatefulSession` | cluster with namespace+port suffix | host stripped, ttl `"3600s"` |
| 3 | `StatefulSession` | `hostname` + `port` | DestinationRule + `# ⚠ MANUAL REVIEW` |
| 4 | `StatefulSession` | `enabled: false` | skipped |
| 5–8 | `LoadBalance` | header / cookie / sourceIp / multi-policy | DestinationRule per mapping |

---

## Lua filters

Skill input: one pair (`HttpFilters` + `RouteConfiguration`). `tests/lua-input.yaml` is an e2e
fixture with **two gateway scenarios** in pre-migration state (no mesh-type guards) — run the
skill per pair and compare with expected output.

```text
Run skill `core-mesh-crs-to-istio` on `tests/lua-input.yaml`.

Compare with `tests/lua-expected-output.yaml`.
```

| # | Gateway | Expected output |
|---|---------|-----------------|
| 1 | `public-gateway-service` | `TrafficExtension` → `public-gateway`, path guard |
| 2 | `internal-gateway-service` | `TrafficExtension` → `waypoint`, path guard |

---

## Egress TLS

Cluster-level `TlsDef` plus a path-based route on `egress-gateway` (`https://` endpoint and
`tlsConfigName`). Compare with [tls-def-mapping.md](../tls-def-mapping.md).

```text
Run skill `core-mesh-crs-to-istio` on `tests/egress-tls-input.yaml`.

Compare with `tests/egress-tls-expected-output.yaml`.
```

| # | Input | Expected output |
|---|-------|-----------------|
| 1 | `TlsDef` `custom-cert` + route prefix `/github` → `https://github.com` | HTTPRoute on Gateway `egress-gateway` (Hostname backend, host rewrite), ServiceEntry, Secret, DestinationRule `tls.mode: SIMPLE` |

---

## Header matchers

One rule per matcher, so the mapping is readable, plus the two Gateway API cannot express.

```text
Run skill `core-mesh-crs-to-istio` on `tests/header-matchers-input.yaml`.

Compare with `tests/header-matchers-expected-output.yaml`.
```

| # | Input | Expected output |
|---|-------|-----------------|
| 1 | `safeRegexMatch` | `type: RegularExpression`, value verbatim — both sides are RE2 full matches |
| 2 | `prefixMatch: v1.2` | `value: v1\.2.*` — metacharacter escaped, `.*` required by full-match semantics |
| 3 | `suffixMatch: .internal` | `value: .*\.internal` |
| 4 | `presentMatch: true` | `value: .*` |
| 5 | `invertMatch: true` | header match dropped, `# ⚠ MANUAL REVIEW` — the rule now matches more than the original |
| 6 | `rangeMatch` | header match dropped, `# ⚠ MANUAL REVIEW` |

Note: the CR's `gateways: ["public-gateway-service"]` is a border gateway name, so the generated
HTTPRoute's `parentRefs` are widened to all three (`public-gateway`, `private-gateway`,
`internal-gateway-service`) per [parent-refs-resolution.md](../parent-refs-resolution.md)
PRIORITY 0 — see `tests/header-matchers-expected-output.yaml`.

---

## Access control — allowed/deny/regExp, and border-gateway widening

Per [docs/istio/allowed-and-forbidden-routes.md](https://github.com/Netcracker/qubership-core-control-plane/blob/main/docs/istio/allowed-and-forbidden-routes.md):
`allowed: false` and `deny: true` rules are never emitted (no 404-rule equivalent on Istio); a
CR on any one border gateway is widened to all three; and `regExp` matches are reduced to
`PathPrefix` roots instead of `RegularExpression`, flagged only when the reduced root is too
general to name an API resource.

```text
Run skill `core-mesh-crs-to-istio` on
`agent-packages/core-mesh-crs-to-istio/.apm/skills/core-mesh-crs-to-istio/tests/access-control-input.yaml`.

Compare the result with `tests/access-control-expected-output.yaml` and report any differences.
Also check the skill's report (`.mesh-migration/reports/core-mesh-crs-to-istio.yaml`) for the
`accessControl` block described in the table below.
```

| # | CR | Input condition | Expected output |
|---|----|-----------------|-----------------|
| 1 | `billing-public-routes` | plain `allowed: true` on `public-gateway-service` | emitted rule; recorded in `accessControl.legacyPublic[]` |
| 2 | `billing-public-routes` | `allowed: false`, no method header matcher | not emitted; `# ⚠ MANUAL REVIEW` (border gateway); recorded in `accessControl.forbidden[]` with no `methods` |
| 3 | `billing-public-routes` | `allowed: false` + `:method` matcher `DELETE` | not emitted; `# ⚠ MANUAL REVIEW`; recorded in `accessControl.forbidden[]` with `methods: [DELETE]` |
| 4 | `billing-public-routes` | `deny: true` | not emitted; `# ⚠ MANUAL REVIEW`; recorded in `accessControl.deny[]` |
| 5 | `billing-public-routes` | `regExp: /api/v1/billing/accounts/[a-f0-9-]+` | reduced to `PathPrefix` root `/api/v1/billing/accounts` — no flag in the YAML; recorded in `accessControl.legacyPublic[]` with `matchType: reduced-regexp` |
| 6 | `billing-public-routes` | `regExp: /api/v1/.*` | reduces to too-general root `/api/v1` — not emitted, `# ⚠ MANUAL REVIEW` |
| 7 | `billing-private-routes` | plain `allowed: true` on `private-gateway-service` | emitted rule, widened `parentRefs` to all three border gateways — **not** added to `accessControl.legacyPublic[]` (CR was never on `public-gateway-service`) |
| 8 | `billing-private-routes` | `deny: true` on `private-gateway-service` | not emitted; `# ⚠ MANUAL REVIEW`; recorded in `accessControl.deny[]` with `gateways: [private-gateway-service]` |

Every `forbidden[]` and `deny[]` entry carries `gateways` (the source CR's `spec.gateways`): rows 2–4
have `[public-gateway-service]`. The report always contains the `accessControl` block; a run on
input with no access-control rules writes it with three empty lists.

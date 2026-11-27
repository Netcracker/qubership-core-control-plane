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
`allowed: false` and `deny: true` rules are never emitted (no 404-rule equivalent on Istio).
A CR that was already on `public-gateway-service` is widened to all three parents. A formerly
non-public CR stays off `public-gateway` until effective access is verified. `regExp` matches are
reduced to `PathPrefix` roots instead of `RegularExpression`. Record every cut. Flag it only when
the cut conflicts (too-general root, or the same normalized match with different behavior).

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
| 2 | `billing-public-routes` | `allowed: false`, no method header matcher | not emitted; the comment says the forbidden rule was omitted and asks whether a remaining rule and policy cover the path; recorded in `accessControl.forbidden[]` with `gateways` and no `methods` |
| 3 | `billing-public-routes` | `allowed: false` + `:method` matcher `DELETE` | not emitted; same omission comment; recorded in `accessControl.forbidden[]` with `methods: [DELETE]` and `gateways: [public-gateway-service]` |
| 4 | `billing-public-routes` | `deny: true` | not emitted; `# ⚠ MANUAL REVIEW` names `public-gateway-service`; recorded in `accessControl.deny[]` with that gateway |
| 5 | `billing-public-routes` | `regExp: /api/v1/billing/accounts/[a-f0-9-]+` | reduced to `PathPrefix` `/api/v1/billing/accounts` — no MANUAL REVIEW; recorded in `legacyPublic[]` with `matchType: reduced-regexp` and in `regexReductions` (`sourceRule`, `original`, `replacement`, `conflicts: false`) |
| 6 | `billing-public-routes` | `regExp: /api/v1/.*` | reduces to too-general root `/api/v1` — not emitted, `# ⚠ MANUAL REVIEW`, `regexReductions[].conflicts: true` |
| 7 | `billing-private-routes` | plain `allowed: true` on `private-gateway-service` | emitted rule with `private-gateway` and `internal-gateway-service` only — **not** `public-gateway`, and **not** added to `accessControl.legacyPublic[]` |
| 8 | `billing-public-routes` | two `/api/v1/billing/reports` rules, headers `X-Tenant: a` and `X-Tenant: b` | both emitted; distinct matches are not deduplicated |
| 9 | `billing-public-routes` | two `/api/v1/billing/exports` rules, timeouts 1000ms and 5000ms | both kept; `# ⚠ MANUAL REVIEW` because the normalized match coincides and the timeouts differ |
| 10 | `billing-egress-deny` | `deny: true` on `egress-gateway` for `/api/v1/billing/invoices` | no HTTPRoute; `accessControl.deny[]` keeps `gateways: [egress-gateway]`. The public policy draft must not include it |

The report always includes `chartPath`, `accessControl.collected: true`, and `publicAttachment`.
An empty list is still a collected inventory. A follow-up run that resolves a skipped gateway
writes manifests only for the skipped CRs and the final `accessControl` lists contain candidates
from both passes.

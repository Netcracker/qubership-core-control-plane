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

---

## Regex routes and forbidden routes

The example of `docs/istio/regex-routes-migration.md` (routes 1–7) on public + private
(`R1`–`R7`) and on internal (`R8`–`R14`). Follows
[regex-routes-migration](../../regex-routes-migration/SKILL.md); its
[worked example](../../regex-routes-migration/worked-example.md) contains the public
worksheet.

```text
Run skill `core-mesh-crs-to-istio` on `tests/regex-routes-input.yaml` (interactive: false).

Compare with `tests/regex-routes-expected-output.yaml`.
```

| # | Input | Expected output |
|---|-------|-----------------|
| 1 | `/resource`, `/resource/{var1}`, `/resource/{var1}/internal-api/status` (same backend, compatible rewrites) | one `PathPrefix /api/v1/my-service/resource` rule, `ReplacePrefixMatch /resource` |
| 2 | `/resource` with `:method: GET` → another-service, next to `/resource/{var1}` | two `Exact` rules (`/resource`, `/resource/`) with `method: GET` and `ReplaceFullPath` |
| 3 | `/resource/{var1}/migrated-api` with rewrite `/resource` | not emitted, `# ⚠ MANUAL REVIEW` (regex rewrite) |
| 4 | `/order/{var1}/items` | `PathPrefix /api/v1/my-service/order`, `ReplacePrefixMatch /order` |
| 5 | `allowed: false` `/resource/{var1}/internal-api` on public + private | DENY rule with `notPaths` for `.../internal-api/status`, in `-deny-public` and `-deny-private` |
| 6 | `/order` exposed by the cut, not routed in legacy | DENY rule `/order` with `notPaths` for `/order/{*}/items` |
| 7 | internal gateway | no AuthorizationPolicy |

Report: `status: complete`, `resources.authorizationPolicy: 2`, no `unresolved:`; `needsReview`
has the two migrated-api lines, the three AuthorizationPolicy precondition lines, and the
line asking to check the worksheet sections.

## Route conflicts

One case per conflict type, then a follow-up run with the answers.

```text
Run skill `core-mesh-crs-to-istio` on `tests/regex-routes-conflicts-input.yaml` (interactive: false).

Compare with `tests/regex-routes-conflicts-expected-output.yaml`, and the report
with `tests/regex-routes-conflicts-expected-report.yaml`.

Then run it again with resolutions
{route-conflict/R1-R2: keep-R1, route-conflict/R3-R4: accept, route-conflict/R5-R6: manual}
and compare with `tests/regex-routes-conflicts-resolved-expected-output.yaml`.
```

| # | Input | First run | After the answers |
|---|-------|-----------|-------------------|
| 1 | R1 `/shop/items` → shop, R2 `/shop/items/{id}/reviews` → reviews | DUP, both held, `default: keep-R1` | R1's rule emitted |
| 2 | R3 `/billing`, R4 `/billing/invoices/{id}/pdf` → `/pdf/invoices/{id}/pdf` | EXPAND, both rules emitted | unchanged |
| 3 | R5 `/catalog/{tenant}/export` → export, R6 `/catalog/public` → catalog | STEAL, both rules emitted; DENY for the exposed `/catalog` | `# ⚠ MANUAL REVIEW` above R5's rule |
| 4 | R8 `allowed: false` `/billing/{id}/audit` on internal | not emitted, `# ⚠ MANUAL REVIEW` (no DENY on internal) | unchanged |

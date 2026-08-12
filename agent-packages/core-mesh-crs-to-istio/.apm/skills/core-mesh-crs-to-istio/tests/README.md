# core-mesh-crs-to-istio — E2E Tests

## Stickiness / load balancing

```
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

```
Run skill `core-mesh-crs-to-istio` on `tests/lua-input.yaml`.

Compare with `tests/lua-expected-output.yaml`.
```

| # | Gateway | Expected output |
|---|---------|-----------------|
| 1 | `public-gateway-service` | `TrafficExtension` → `public-gateway`, path guard |
| 2 | `internal-gateway-service` | `TrafficExtension` → `waypoint`, path guard |

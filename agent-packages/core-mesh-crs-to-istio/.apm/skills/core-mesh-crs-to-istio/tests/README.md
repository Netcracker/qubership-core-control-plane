# core-mesh-crs-to-istio — E2E Test

## How to run

Open a new chat and send:

```
Run skill `core-mesh-crs-to-istio` on file
`agent-packages/core-mesh-crs-to-istio/.apm/skills/core-mesh-crs-to-istio/tests/input.yaml`.

Compare the result with `tests/expected-output.yaml` from the same folder and report any differences.
```

## Scenarios covered

| # | CR | Input condition | Expected output |
|---|----|-----------------|-----------------|
| 1 | `StatefulSession` | cookie, ttl=0, simple cluster name | DestinationRule with `httpCookie`, ttl `"0s"` |
| 2 | `StatefulSession` | cluster with namespace+port suffix, ttl=3600 | host stripped to service name, ttl `"3600s"` |
| 3 | `StatefulSession` | `hostname` + `port` set | DestinationRule generated + `# ⚠ MANUAL REVIEW` comment |
| 4 | `StatefulSession` | `enabled: false` | skipped — no DestinationRule |
| 5 | `LoadBalance` | single header policy | DestinationRule with `httpHeaderName` |
| 6 | `LoadBalance` | single cookie policy, ttl=0 | DestinationRule with `httpCookie`, ttl `"0s"` |
| 7 | `LoadBalance` | single sourceIp policy | DestinationRule with `useSourceIp: true` |
| 8 | `LoadBalance` | two policies | first policy used + `# ⚠ MANUAL REVIEW` comment listing dropped policy |

## What to check

- Host resolution: `cluster.namespace:port` → bare service name
- TTL formatting: `null`/`0` → `"0s"`, `N` → `"Ns"`
- Istio guard wraps the entire output file
- MANUAL REVIEW comments present where required
- Disabled/empty StatefulSession produces no output

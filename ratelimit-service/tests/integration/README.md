# tests/integration/

Integration tests that use miniredis (in-process Redis). They require no kubectl or port-forward and run in isolation.

## Files

| File | What it tests |
|------|---------------|
| `ratelimit_test.go` | RateLimitManager: Check, GetViolatingUsers |
| `controller_test.go` | ConfigMapController + RateLimitManager |
| `api_test.go` | HTTP server (api.Server.CheckRateLimit) via httptest |

## Running

```bash
make test-integration
```

## Notes

- **metrics_test.go is in tests/e2e/** (as `metrics_collector_test.go`) — `metrics.NewMetricsCollectorService` requires Redis `INFO` commands that miniredis does not support. The test skips itself if a real Redis is unavailable.
- All tests here use `helpers.NewEnv(t)` to obtain a miniredis environment.
- Build tag: `//go:build integration`

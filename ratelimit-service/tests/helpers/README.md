# tests/helpers/

Reusable Go test code: miniredis factories, YAML fixture loading, free-port allocation, test ConfigMap creation.

- `miniredis.go` — standard miniredis + RedisClient + RateLimitManager setup.
- `fixtures.go` — load YAML from `tests/fixtures/`, parse into `[]*ratelimit.Rule`.
- `ports.go` — allocate a free TCP port for tests.
- `configmap.go` — create a test ConfigMap via `fake.Clientset` or a real k8s API; `SetRules(t, clientset, namespace, rules)` with `t.Cleanup`.

Helpers are used from `tests/integration/`, `tests/e2e/`, and `tests/cloud-e2e/`.

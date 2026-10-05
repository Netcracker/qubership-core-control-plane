# httproute-from-code — E2E Tests

## Forbidden routes and path variables

`routes.go` registers four Go routes. They exercise
[regex-routes-migration](../../regex-routes-migration/SKILL.md): implicit
forbidden rows on the gateways wider than a route's type, an explicit
`Forbidden: true` route, the `PathPrefix` cut, and the exposure DENY rule.

```text
Run skill `httproute-from-code` on `tests/routes.go` with interactive: false,
backendRefName: demo-service, backendRefPort: 8080,
routeLabels: {app.kubernetes.io/name: demo-service},
resolutions: {microservice-name: demo-service}.

Compare the generated source-code-httproutes.yaml with `tests/expected-output.yaml`.
```

| # | Route | Expected output |
|---|-------|-----------------|
| R1 | Public `/api/v1/svc/{id}` → `/svc/{id}` | public CR rule `PathPrefix /api/v1/svc` → `/svc`; exposure DENY `/api/v1/svc` (bare path) on public and private, `notPaths` `/api/v1/svc/{*}` |
| R2 | Internal `/api/v1/svc/{id}/admin` → `/svc/{id}/admin` | merged into R1's rule (internal); DENY `/api/v1/svc/{*}/admin` on public and private — implicit, covered by R1's cut |
| R3 | Private `/api/v1/svc/orders/{id}/items`, 30s | private CR rule `PathPrefix /api/v1/svc/orders` → `/svc/orders`, timeout 30s; DENY on public (implicit); `notPaths` on private |
| R4 | Public `/api/v1/svc/debug`, `Forbidden: true` | DENY on public and private; rule without `backendRefs` for internal, with `# ⚠ MANUAL REVIEW` because it shadows `/api/v1/svc/debug/admin` (R2 in legacy) |

Report: `status: complete`, `routesGenerated: 3`, `authorizationPoliciesGenerated: 2`, no
`unresolved:`; `needsReview` has the R4 note, the three AuthorizationPolicy precondition
lines, and the line asking to check the worksheet sections.

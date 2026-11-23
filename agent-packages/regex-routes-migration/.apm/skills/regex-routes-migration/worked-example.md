# Worked example

Part of the [`regex-routes-migration`](SKILL.md) skill: the full run of Steps 1–8 for one RouteConfiguration.

Legacy routes of one RouteConfiguration attached to `public-gateway-service` and
`private-gateway-service` (identical worksheets — public shown). Backends:
`my-service:8080` = `B1`, `another-service:8080` = `B2`.

| # | path | kind | headers | allowed | forbidden | behavior | rewrite | L | cut | T | R | emit |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| R1 | `/api/v1/my-service/resource` | literal | - | yes | - | B1 | `/resource` | 27 | `/api/v1/my-service/resource` | `/api/v1/my-service/resource` | `/resource` | rule |
| R2 | `/api/v1/my-service/resource/{var1}/internal-api` | variable | - | no | explicit | - | - | 42 | `/api/v1/my-service/resource` | `/api/v1/my-service/resource/{*}/internal-api` | - | deny |
| R3 | `/api/v1/my-service/resource/{var1}/migrated-api` | variable | - | yes | - | B2 | `/resource` | 42 | `/api/v1/my-service/resource` | `/api/v1/my-service/resource/{*}/migrated-api` | ✗ | review |
| R4 | `/api/v1/my-service/resource` | literal | `:method=GET` | yes | - | B2 | `/resource` | 27 | `/api/v1/my-service/resource` | `/api/v1/my-service/resource` | `/resource` | exact |
| R5 | `/api/v1/my-service/resource/{var1}` | variable | - | yes | - | B1 | `/resource/{var1}` | 29 | `/api/v1/my-service/resource` | `/api/v1/my-service/resource/{*}` | `/resource` | merged→R1 |
| R6 | `/api/v1/my-service/resource/{var1}/internal-api/status` | variable | - | yes | - | B1 | `/resource/{var1}/internal-api/status` | 49 | `/api/v1/my-service/resource` | `/api/v1/my-service/resource/{*}/internal-api/status` | `/resource` | merged→R1 |
| R7 | `/api/v1/my-service/order/{var1}/items` | variable | - | yes | - | B1 | `/order/{var1}/items` | 32 | `/api/v1/my-service/order` | `/api/v1/my-service/order/{*}/items` | `/order` | rule |
| X1 | `/api/v1/my-service/order` | literal | - | no | exposure | - | - | 24 | `/api/v1/my-service/order` | `/api/v1/my-service/order` | - | deny |

How each decision was made:

- **Step 2** — R3: `S` = `{var1}/migrated-api` (2 segments), `rewrite` `/resource`
  has 1 segment → `✗`, `review`. R5: `S` = `{var1}`, last segment of
  `/resource/{var1}` is `{var1}` → `R` = `/resource`. R6, R7 likewise.
- **Step 3** — group (`/api/v1/my-service/resource`, `-`): R1, R5, R6 are
  `sameBehavior` (B1, `R` `/resource`) → one rule.
- **Step 4** — R4 has `:method=GET`, R5 = R4.path + `/{var1}` without headers,
  behaviors differ → R4 `exact`.
- **Step 5** — no conflicts: R3 is `review`, R4 is `exact`, R5/R6 merged with R1,
  R7 shares no `cut` with another row.
- **Step 6a** — R2 is explicit → `deny`.
- **Step 6b** — R5/R6: R1 covers `/api/v1/my-service/resource`. R7: nothing covers
  `/api/v1/my-service/order` → exposure row X1.
- **Step 6c** — R2: candidates below it that win: R6 (`L` 49 > 42) → `notPaths`
  R6. R3 is not below (`migrated-api` ≠ `internal-api`), R5 has fewer segments.
  X1: candidate R7 (`L` 32 > 24).

On `internal-gateway-service` (a second RouteConfiguration with the same routes,
where the internal API is allowed: `/resource/{var1}/internal-api` →
`/resource/{var1}/internal-api`, B1) the internal-API row merges into R1, R3 is
`review`, R4 is `exact`, R7 is a `rule` — and Step 7 generates nothing else.

Rendered for the public/private HTTPRoute (rules sorted by the consumer):

```yaml
  rules:
  # ⚠ MANUAL REVIEW: /api/v1/my-service/resource/{var1}/migrated-api needs a regex rewrite
  # (rewrite /resource); not emitted — its requests now reach the rule that owns
  # /api/v1/my-service/resource (my-service).
  - matches:
    - path:
        type: Exact
        value: /api/v1/my-service/resource/
      method: GET
    filters:
    - type: URLRewrite
      urlRewrite:
        path:
          type: ReplaceFullPath
          replaceFullPath: /resource/
    backendRefs:
    - group: ''
      kind: Service
      name: another-service
      port: 8080
      weight: 1
  - matches:
    - path:
        type: Exact
        value: /api/v1/my-service/resource
      method: GET
    filters:
    - type: URLRewrite
      urlRewrite:
        path:
          type: ReplaceFullPath
          replaceFullPath: /resource
    backendRefs:
    - group: ''
      kind: Service
      name: another-service
      port: 8080
      weight: 1
  - matches:
    - path:
        type: PathPrefix
        value: /api/v1/my-service/resource
    filters:
    - type: URLRewrite
      urlRewrite:
        path:
          type: ReplacePrefixMatch
          replacePrefixMatch: /resource
    backendRefs:
    - group: ''
      kind: Service
      name: my-service
      port: 8080
      weight: 1
  - matches:
    - path:
        type: PathPrefix
        value: /api/v1/my-service/order
    filters:
    - type: URLRewrite
      urlRewrite:
        path:
          type: ReplacePrefixMatch
          replacePrefixMatch: /order
    backendRefs:
    - group: ''
      kind: Service
      name: my-service
      port: 8080
      weight: 1
```

And the public policy (the private one is identical with `-deny-private` and
`private-gateway`):

```yaml
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: my-service-routes-deny-public
spec:
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: Gateway
    name: public-gateway
  action: DENY
  rules:
  - to:
    - operation:
        ports: ["8080"]
        paths:
        - "/api/v1/my-service/order"
        - "/api/v1/my-service/order/{**}"
        notPaths:
        - "/api/v1/my-service/order/{*}/items"
        - "/api/v1/my-service/order/{*}/items/{**}"
  - to:
    - operation:
        ports: ["8080"]
        paths:
        - "/api/v1/my-service/resource/{*}/internal-api"
        - "/api/v1/my-service/resource/{*}/internal-api/{**}"
        notPaths:
        - "/api/v1/my-service/resource/{*}/internal-api/status"
        - "/api/v1/my-service/resource/{*}/internal-api/status/{**}"
```

These are the same DENY rules `httproutes-generator-maven-plugin` generates from
the equivalent annotated Java controllers.


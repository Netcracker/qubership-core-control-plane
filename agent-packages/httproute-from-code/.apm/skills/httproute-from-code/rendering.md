# Steps 10–12 — Generate, format and summarize

Part of the [`httproute-from-code`](SKILL.md) skill. Render only what the worksheets of Step 4 decided (see [`SKILL.md`](SKILL.md)).

## Step 10 — Generate HTTPRoute

Generate one CR per RouteType. Wrap ALL CRs together in a single Istio conditional block.

**Rule order:** emit `rules[]` in the path-specificity order produced by
[Step 9](SKILL.md#step-9--sort-rules-by-path-specificity) (shared procedure
[`path-specificity-sorting.md`](../path-specificity-sorting/SKILL.md)).
Most specific match first — never in source/discovery order.

### ParentRef resolution

Resolve every target gateway name to a Gateway API `parentRefs` entry before rendering:

| Target | Rendered parentRef |
|---|---|
| `public-gateway` | `- name: public-gateway` with `kind: Gateway` and `group: gateway.networking.k8s.io` |
| `private-gateway` | `- name: private-gateway` with `kind: Gateway` and `group: gateway.networking.k8s.io` |
| `internal-gateway-service` | `- name: internal-gateway-service` with `kind: Service` and `group: ''` |

**Mandatory fields — every `parentRefs[]` entry MUST render all three:**

- `group:` — `gateway.networking.k8s.io` for `kind: Gateway`, or `''` (empty
  string) for `kind: Service`. Always present, never omitted.
- `kind:` — `Gateway` or `Service`.
- `name:` — the resolved parent name.

Never emit a parentRef with a missing `group`, `kind`, or `name` (an empty
`group` must still be written as `group: ''`, not dropped).

### BackendRef resolution

**Mandatory fields — every rule's `backendRefs[]` entry MUST render all five:**

- `group:` — always `''` (empty string), never omitted.
- `kind:` — always `Service`.
- `name:` ← `backendRefName` (default `{{ .Values.DEPLOYMENT_RESOURCE_NAME }}`).
- `port:` ← `backendRefPort` (default `8080`).
- `weight:` — always `1`.

The same `backendRefName` / `backendRefPort` apply to every rule across every CR
— they are migration-wide, not per-route (see
[Inputs / parameters](SKILL.md#inputs--parameters)). The examples below use the defaults;
substitute the confirmed values when they differ.

These five fields are required on every emitted rule, with one exception: a
forbidden route without variables on a gateway that gets no DENY policy
(internal, mesh, facade) is rendered as a rule **without** `backendRefs`, per
regex-routes-migration Step 7 — Istio answers it with 404, as legacy did.

### Labels resolution

If `routeLabels` is provided:

- Render a `metadata.labels` section on every generated HTTPRoute CR.
- Copy all labels exactly as provided (including Helm template expressions).
- Keep the same label set for all generated CRs in this run.

If `routeLabels` is not provided:

- Render `metadata.labels` using the default label set from
  [Inputs / parameters](SKILL.md#inputs--parameters).

### HTTPRoute naming schema

Generated HTTPRoute names follow this fixed pattern:

`<microservice-name>-source-code-<route-type>-routes`

Where `<route-type>` is lowercase and mapped as:

| Canonical RouteType | Name suffix |
|---|---|
| `Public` | `public-routes` |
| `Private` | `private-routes` |
| `Internal` | `internal-routes` |
| `Mesh` | `mesh-routes` |
| `Facade` | `facade-routes` |

Examples:

- `billing-service-source-code-public-routes`
- `billing-service-source-code-private-routes`
- `billing-service-source-code-internal-routes`

Naming rules:

- Use the microservice name resolved in [Step 7](SKILL.md#step-7--resolve-microservice-name).
- Emit exactly one HTTPRoute name per RouteType that has routes (see Step 6).
- If Step 7 cannot resolve the service name, use `<microservice-name>` in the
  generated name and record the `unresolved:` entry per Step 7 — the caller
  supplies the real name via the `resolutions` input.

```yaml
{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: <microservice-name>-source-code-public-routes
  labels:
    app.kubernetes.io/name: {{ .Values.SERVICE_NAME }}
    app.kubernetes.io/part-of: {{ .Values.APPLICATION_NAME }}
spec:
  parentRefs:
    - group: gateway.networking.k8s.io    
      kind: Gateway
      name: public-gateway
    - group: gateway.networking.k8s.io    
      kind: Gateway    
      name: private-gateway
    - group: ''
      kind: Service
      name: internal-gateway-service

  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /api/v1/mesh-test-service-go
      filters:
        - type: URLRewrite
          urlRewrite:
            path:
              type: ReplacePrefixMatch
              replacePrefixMatch: /api/v1
      backendRefs:
        - group: ''
          kind: Service
          name: {{ .Values.DEPLOYMENT_RESOURCE_NAME }}
          port: 8080
          weight: 1
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: <microservice-name>-source-code-private-routes
spec:
  parentRefs:
    - group: gateway.networking.k8s.io    
      kind: Gateway    
      name: private-gateway
    - group: ''
      kind: Service
      name: internal-gateway-service
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /api/v1/mesh-test-service-go-private
      filters:
        - type: URLRewrite
          urlRewrite:
            path:
              type: ReplacePrefixMatch
              replacePrefixMatch: /api/v1/private
      backendRefs:
        - group: ''
          kind: Service
          name: {{ .Values.DEPLOYMENT_RESOURCE_NAME }}
          port: 8080
          weight: 1
{{- end }}
```

The `{{- if eq .Values.SERVICE_MESH_TYPE "Istio" }}` opens before the first CR and `{{- end }}` closes after the last CR. The `---` separators between CRs remain inside the block.

### AuthorizationPolicy

After the HTTPRoute CRs, inside the same Istio block, render one
`AuthorizationPolicy` per border gateway that has DENY rules (regex-routes-migration
Steps 6 and 8):

| Gateway | Name | `targetRefs` |
|---|---|---|
| `public-gateway-service` | `<microservice-name>-source-code-deny-public` | Gateway `public-gateway` |
| `private-gateway-service` | `<microservice-name>-source-code-deny-private` | Gateway `private-gateway` |

All DENY rules of a gateway go into its one policy (every rule of this skill has
the same owner service) — implicit, explicit and exposure rules alike, copied
from that gateway's `## DENY rules` worksheet section. Render it with the
AuthorizationPolicy template of regex-routes-migration Step 8 and nothing else
(`security.istio.io/v1`, no `from`, `ports: ["8080"]`). Labels: the same
`routeLabels` as the HTTPRoutes. Add the three precondition lines of
regex-routes-migration Step 8 to `needsReview` once.

---

## Step 11 — Output formatting

Separate multiple CRs with `---`.

Output file:

```
helm-templates/<service name>/templates/source-code-httproutes.yaml
```

---

## Step 12 — Summary

```
## Summary

| # | File | From | To | RouteType | Gateways | Timeout | Hosts | Emitted as |
|---|---|---|---|---|---|---|---|---|
| R1 | routes.go | /api/v1/users/{id}/profile | /users/{id}/profile | Public | public,private,internal | 30s | - | PathPrefix /api/v1/users → /users |
| R2 | RouteConfig.java | /api/v1/users | /users | Public | public,private,internal | - | - | PathPrefix /api/v1/users → /users |
| R3 | routes.go | /mesh | /mesh | Mesh | mesh-gateway | - | - | PathPrefix /mesh → /mesh |
| R4 | RouteConfig.java | /api/v1/users/admin | /users/admin | Public | - | - | - | DENY on public, private; no-backend rule on internal (forbidden) |
```

Note: summary rows reflect sorted order (most specific first). List the
AuthorizationPolicies, their rules, and every route conflict below the table.

Also report the `backendRefs` values applied to all rules:

```
backendRefName: {{ .Values.DEPLOYMENT_RESOURCE_NAME }}   (detected | user-provided | default)
backendRefPort: 8080                                     (detected | user-provided | default)
```

Also report labels applied to generated CRs:

```
routeLabels: <map or "default label set">
```

---


# Rule-level StatefulSession → DestinationRule

Applies when `statefulSession` is set on a `RouteV3.Rule` inside a `RouteConfiguration`.

> **Out of scope:** standalone `StatefulSession` / `LoadBalance` CR documents are handled by the
> separate `core-mesh-crs-to-istio` skill, not by this file.

Target:

```yaml
apiVersion: networking.istio.io/v1
kind: DestinationRule
```

---

## Procedure

- **Destination host:** parsed from the rule's `RouteDestination.endpoint` — same parsing as
  [route-configuration-mapping.md](route-configuration-mapping.md) → "Endpoint to backendRef resolution".
- Generate **one DestinationRule per destination host**.
- **Multiple rules with `statefulSession` for the same host:** the first configuration wins; add a
  `# ⚠ MANUAL REVIEW` comment listing each subsequent conflicting rule.
- **Output placement:** written in the **same generated `-istio` file** as the HTTPRoute, after the
  HTTPRoute document (separated by `---`), inside the same Istio guard.

---

## StatefulSession field mapping (rule-level)

| JSON key | Go type | Transformation |
|---|---|---|
| `enabled` | `*bool` | if `false` → skip, do not generate a DestinationRule |
| `cookie` | `*Cookie` | → `spec.trafficPolicy.loadBalancer.consistentHash.httpCookie` (see Cookie below) |
| other fields | — | OMIT (`version`, `namespace`, `cluster`, `hostname`, `gateways`, `port`, `route`, `overridden`) |

If `cookie` is absent → delete/disable request; do **not** generate a DestinationRule.

### Cookie

| JSON key | Go type | Transformation |
|---|---|---|
| `name` | `string` | → `httpCookie.name` |
| `ttl` | `*int64` | → `httpCookie.ttl` (format `"Ns"`; `null` or `0` → `"0s"` — session cookie) |
| `path` | `string` | → `httpCookie.path` (omit if empty) |

---

## Output example

Input (rule inside a `RouteConfiguration`):

```yaml
rules:
  - match:
      prefix: /api/v1/trace
    statefulSession:
      enabled: true
      cookie:
        name: trace-service-sticky
        ttl: 0
        path: /
```

Output (appended after the HTTPRoute in the same `-istio` file):

```yaml
---
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: <httproute-name>-sticky
  namespace: {{ .Release.Namespace }}
spec:
  host: <host parsed from RouteDestination.endpoint>
  trafficPolicy:
    loadBalancer:
      consistentHash:
        httpCookie:
          name: trace-service-sticky
          ttl: "0s"
          path: /
```

## StatefulSession → DestinationRule

Source:

    apiVersion: nc.core.mesh/v3
    kind: StatefulSession

Target:

    apiVersion: networking.istio.io/v1
    kind: DestinationRule

---

### StatefulSession.spec

  JSON key   Go type          Transformation
  ─────────────────────────────────────────────────────────────────────────────────────────
  cluster    string           → spec.host  (see Host resolution)
  namespace  string           OMIT
  version    string           OMIT
  hostname   string           OMIT ⚠ flag for MANUAL REVIEW if non-empty (see Endpoint-level targeting below)
  port       int              OMIT ⚠ flag for MANUAL REVIEW if non-empty (see Endpoint-level targeting below)
  gateways   []string         OMIT
  enabled    *bool            if false → skip, do not generate DestinationRule
  cookie     *Cookie          → spec.trafficPolicy.loadBalancer.consistentHash.httpCookie  (see Cookie below)
  route      *RouteMatcher    OMIT (read-only response field)
  overridden bool             OMIT ⚠ flag for MANUAL REVIEW if non-empty

If `cookie` is absent → delete/disable request; do **not** generate a DestinationRule.

---

### Cookie

  JSON key  Go type  Transformation
  ────────────────────────────────────────────────────────────────────────────────────
  name      string   → spec.trafficPolicy.loadBalancer.consistentHash.httpCookie.name
  ttl       *int64   → spec.trafficPolicy.loadBalancer.consistentHash.httpCookie.ttl  (format: "Ns"; null or 0 → "0s")
  path      string   → spec.trafficPolicy.loadBalancer.consistentHash.httpCookie.path  (omit if empty)

---

### Host resolution

Extract the service name from `spec.cluster` — strip namespace suffix and port:

    "service"               → "service"
    "service.namespace"     → "service"
    "service:8080"          → "service"

---

### TTL formatting

  Source ttl  Istio ttl
  ─────────────────────
  null        "0s"   (session cookie)
  0           "0s"   (session cookie)
  3600        "3600s"

---

### Endpoint-level targeting

Control-plane allows a `StatefulSession` to pin stickiness to a **specific endpoint** (pod) via `hostname`/`port`.
Istio `DestinationRule.spec.host` targets a Kubernetes Service (all pods behind it), so the same precision
cannot be expressed without a separate Service per endpoint group.

**Migration path when `hostname` or `port` is set:**

1. Check whether a dedicated Kubernetes Service already exists for that endpoint (e.g. `trace-service-v1`).
2. If yes — generate the DestinationRule with `spec.host` pointing to that Service instead of the base cluster name.
3. If no — flag for MANUAL REVIEW: create the Service first, then re-run the migration for this CR.

Do **not** silently fall back to cluster-level DR — add a `# ⚠ MANUAL REVIEW` comment explaining the situation.

---

### Rule-level StatefulSession (inside RouteConfiguration)

When `statefulSession` appears on a `RouteV3.Rule` inside a `RouteConfiguration`,
generate a DestinationRule for the route's destination host alongside the HTTPRoute.

- Destination host: parsed from `RouteDestination.endpoint` (same as backendRef resolution)
- Cookie mapping: same as above
- Multiple rules for the same host: first config wins, add `# ⚠ MANUAL REVIEW` comment for subsequent conflicts
- Output: written **in the same generated file** as the HTTPRoute, after the HTTPRoute document (separated by `---`)

---

### Output example

Input:
```yaml
apiVersion: nc.core.mesh/v3
kind: StatefulSession
metadata:
  name: trace-service-sticky
  namespace: trace-namespace
spec:
  gateways: ["public-gateway-service"]
  cluster: trace-service
  version: v1
  enabled: true
  cookie:
    name: trace-service-sticky
    ttl: 0
    path: /
```

Output:
```yaml
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: trace-service-sticky
  namespace: trace-namespace
spec:
  host: trace-service
  trafficPolicy:
    loadBalancer:
      consistentHash:
        httpCookie:
          name: trace-service-sticky
          ttl: "0s"
  
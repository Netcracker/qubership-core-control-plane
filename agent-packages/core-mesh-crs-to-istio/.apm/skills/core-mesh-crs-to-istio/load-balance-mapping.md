## LoadBalance → DestinationRule

Source:

    apiVersion: nc.core.mesh/v2   (or nc.core.mesh/v3)
    kind: LoadBalance

Target:

    apiVersion: networking.istio.io/v1
    kind: DestinationRule

---

### LoadBalance.spec

  JSON key   Go type        Transformation
  ──────────────────────────────────────────────────────────────────────────
  cluster    string         → spec.host  (see Host resolution)
  namespace  string         OMIT
  version    string         OMIT
  endpoint   string         OMIT
  overridden bool           OMIT ⚠ flag for MANUAL REVIEW if non-empty
  policies   []HashPolicy   → spec.trafficPolicy.loadBalancer.consistentHash  (see HashPolicy below)

If `policies` is empty or absent → do **not** generate a DestinationRule.

---

### HashPolicy

Istio `consistentHash` supports exactly **one** hash type per DestinationRule.
Only the **first** policy is used. If multiple policies are present, generate the DR from
the first policy and add a `# ⚠ MANUAL REVIEW` comment listing the dropped policies.

Exactly one of the following sub-fields must be set per HashPolicy:

  JSON key                           Go type  Transformation
  ─────────────────────────────────────────────────────────────────────────────────────────────────────
  header.headerName                  string   → consistentHash.httpHeaderName
  cookie.name / cookie.ttl / cookie.path      → consistentHash.httpCookie  (see Cookie below)
  connectionProperties.sourceIp=true bool     → consistentHash.useSourceIp: true
  queryParameter.name                string   → consistentHash.httpQueryParameterName
  terminal                           bool     OMIT (no Istio equivalent)

---

### Cookie (inside HashPolicy)

  JSON key  Go type  Transformation
  ────────────────────────────────────────────────────────────────────────────────────
  name      string   → consistentHash.httpCookie.name
  ttl       *int64   → consistentHash.httpCookie.ttl  (format: "Ns"; null or 0 → "0s")
  path      string   → consistentHash.httpCookie.path  (omit if empty)

---

### Host resolution

Same as StatefulSession: extract service name from `spec.cluster`, strip namespace/port suffix.

    "trace-service"                      → "trace-service"
    "trace-service.trace-namespace"      → "trace-service"
    "trace-service:8080"                 → "trace-service"

---

### TTL formatting

  Source ttl  Istio ttl
  ─────────────────────
  null        "0s"   (session cookie)
  0           "0s"   (session cookie)
  N           "Ns"

---

### Output examples

**Header-based:**
```yaml
# Input spec.policies[0]
- header:
    headerName: X-User-Id
```
```yaml
# Output
consistentHash:
  httpHeaderName: X-User-Id
```

**Cookie-based:**
```yaml
# Input spec.policies[0]
- cookie:
    name: LB_COOKIE
    ttl: 0
    path: /
```
```yaml
# Output
consistentHash:
  httpCookie:
    name: LB_COOKIE
    ttl: "0s"
    path: /
```

**Source IP:**
```yaml
# Input spec.policies[0]
- connectionProperties:
    sourceIp: true
```
```yaml
# Output
consistentHash:
  useSourceIp: true
```

**Query parameter:**
```yaml
# Input spec.policies[0]
- queryParameter:
    name: session_id
```
```yaml
# Output
consistentHash:
  httpQueryParameterName: session_id
```

**Multiple policies (first wins):**
```yaml
# Input
policies:
  - header:
      headerName: BID
  - cookie:
      name: JSESSION
      ttl: 5
```
```yaml
# Output
# ⚠ MANUAL REVIEW: LoadBalance had 2 policies; only the first (header: BID) was migrated.
#   Dropped: cookie: JSESSION — Istio DestinationRule supports only one consistentHash type.
spec:
  host: trace-service
  trafficPolicy:
    loadBalancer:
      consistentHash:
        httpHeaderName: BID
```

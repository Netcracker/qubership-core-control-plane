## Egress TLS: TlsDef + egress RouteConfiguration → Secret, ServiceEntry, DestinationRule

Core Mesh egress TLS is **outbound origination** at `egress-gateway`: mesh clients send HTTP to
the gateway; the gateway opens HTTPS (or mTLS) to the external host. It is not inbound TLS on
the Gateway listener. Keep the Istio Gateway listener HTTP. Do not emit `BackendTLSPolicy`.

Preserve Core Mesh's path-based API: clients call the egress gateway with a path prefix
(`/github`), not the external hostname. Do not rewrite this into host-based transparent egress
(client → `github.com` via `istio.io/use-waypoint` on a ServiceEntry).

Source (often in one apply, `TlsDef` first):

```text
apiVersion: nc.core.mesh/v3
kind: TlsDef                 — named TLS profile (CA, optional client cert, SNI, insecure)

apiVersion: nc.core.mesh/v3
kind: RouteConfiguration     — spec.gateways includes an egress gateway;
                               RouteDestination.endpoint is https://…;
                               RouteDestination.tlsConfigName selects a cluster-level TlsDef
```

Or `core.netcracker.com/v1` `Mesh` with `subKind: TlsDef` / `subKind: RouteConfiguration`
(identical `spec` shape). `TlsDef` is usually `nc.core.mesh/v3` with identity in `spec.name`
(metadata.name may be absent).

Targets (all in the Istio-guarded sibling; Secret may live in the TlsDef sibling):

| Source | Output |
|---|---|
| External egress destination | `ServiceEntry` (`MESH_EXTERNAL`) + HTTPRoute `backendRef` `kind: Hostname` |
| `TlsDef` cert material | `Secret` (`stringData`; preserve Helm expressions) |
| Resolved TLS profile | `DestinationRule` `trafficPolicy.tls` with `credentialName` |

---

### Core Mesh TLS levels and priority

Match Core Mesh egress TLS (`TlsDef` at gateway vs cluster level):

| Level | How you recognize it | SNI | Scope |
|---|---|---|---|
| Cluster (non-default) | `TlsDef` has no `trustedForGateways`; route sets `tlsConfigName` to `spec.name` | allowed | that destination only |
| Gateway | `trustedForGateways` contains `egress-gateway` (only gateway Core Mesh allows) | forbidden | every cluster on that gateway that has no non-default cluster TlsDef |
| Cluster (default) | `spec.name` equals `<cluster>-tls` (control-plane generated) | allowed | that cluster; lowest priority |

Priority, highest first: non-default cluster TlsDef → gateway TlsDef → default cluster TlsDef.

A gateway-level TlsDef must not include `tls.sni`. If it does, omit SNI from that profile and
add `# ⚠ MANUAL REVIEW`. Istio origination still sets `sni` to the **destination hostname**
(see DestinationRule below). That is the Istio stand-in for Core Mesh's "no SNI on the
gateway profile" rule.

Same `spec.name` used once as cluster-level and once as gateway-level overwrites in Core Mesh.
If both appear, keep the cluster-level profile for destinations that reference it, and add
`# ⚠ MANUAL REVIEW`.

`tls.enabled: false` or empty `tls:` → do not emit Secret or DestinationRule for that profile
(disable / delete). Still wrap the source TlsDef in the Core guard.

---

### When a destination is an egress external host

The RouteConfiguration attaches to a resolved **egress** gateway (`egress-gateway` by name, or
`spec.gatewayType: egress`), **and** any of:

- `endpoint` scheme is `https://`
- `tlsConfigName` is non-empty
- parsed host contains `.` (FQDN) or is a Helm expression that is not a bare service name

Otherwise keep the in-cluster Service `backendRef` in
[route-configuration-mapping.md](route-configuration-mapping.md).

Mixed ingress/mesh and egress names on one `spec.gateways` list → `# ⚠ MANUAL REVIEW`; still
apply this mapping only to the egress parentRefs.

---

### TlsDef.spec

  JSON key              Go type     Transformation
  ──────────────────────────────────────────────────────────────────────────────
  name                  string      → Secret.metadata.name and DestinationRule
                                      `credentialName`; identity of the profile
  trustedForGateways    []string    → classifies gateway-level vs cluster-level;
                                      OMIT from output. Non-egress values →
                                      `# ⚠ MANUAL REVIEW`
  overridden            bool        OMIT ⚠ flag if true
  tls                   *Tls        → Secret + DestinationRule (see Tls)
  tls.enabled           bool        false → skip Secret and DestinationRule
  tls.insecure          bool        → `insecureSkipVerify: true`; no Secret if
                                      there is no cert material
  tls.sni               string      → DestinationRule `trafficPolicy.tls.sni`
                                      (cluster-level only)
  tls.trustedCA         string      → Secret `stringData.ca.crt`
  tls.clientCert        string      → Secret `stringData.tls.crt` (with privateKey)
  tls.privateKey        string      → Secret `stringData.tls.key` (with clientCert)

Empty `trustedCA` while `insecure: false` is invalid in Core Mesh → `# ⚠ MANUAL REVIEW`, skip.
`clientCert` without `privateKey` or the reverse → `# ⚠ MANUAL REVIEW`, skip mTLS (do not
emit a half-filled Secret).

Resolve the profile for each egress destination in this order:

1. `tlsConfigName` matches a cluster-level TlsDef `spec.name` in the scanned chart.
2. Else a gateway-level TlsDef whose `trustedForGateways` includes this gateway.
3. Else a TlsDef named `<destination.cluster>-tls`.
4. Else if scheme is `https` and no TlsDef: DestinationRule `tls.mode: SIMPLE` with no
   `credentialName` (Istio system CAs) and `# ⚠ MANUAL REVIEW` (Core Mesh required an
   explicit CA or `insecure: true`).
5. `tlsConfigName` set but no TlsDef in the chart → HTTPRoute + ServiceEntry only,
   `# ⚠ MANUAL REVIEW`.

A TlsDef in the chart that no destination consumes → emit the Secret only,
`# ⚠ MANUAL REVIEW`.

---

### Tls → DestinationRule.trafficPolicy.tls

| TlsDef | `tls.mode` | Secret keys | Other fields |
|---|---|---|---|
| `insecure: true` | `SIMPLE` | none | `insecureSkipVerify: true`; omit `credentialName` |
| `trustedCA` only | `SIMPLE` | `ca.crt` | `credentialName: <spec.name>` |
| `trustedCA` + `clientCert` + `privateKey` | `MUTUAL` | `ca.crt`, `tls.crt`, `tls.key` | `credentialName: <spec.name>` |

`sni`:

- Cluster-level: copy `tls.sni` when set; otherwise the destination hostname.
- Gateway-level: always the destination hostname (Core Mesh forbids SNI on this profile).

One DestinationRule per **external host** (Istio `spec.host` is the ServiceEntry host, not
the Core Mesh `cluster` token). Gateway-level TlsDef expands to one DestinationRule per
egress host that did not take a cluster-level profile. Name:

- cluster-level: `<tls.spec.name>` (example: `custom-cert`)
- gateway-level expansion: `<tls.spec.name>-<host-with-dots-as-dashes>`

If a load-balancing or sticky-session DestinationRule already targets the same `spec.host`,
merge `trafficPolicy.tls` into that document (one DestinationRule per host). Conflict on
`tls` itself → keep origination, `# ⚠ MANUAL REVIEW`.

`credentialName` is valid because origination runs on the egress Gateway (Envoy), not on a
sidecar.

---

### ServiceEntry

One ServiceEntry per distinct external hostname in the chart.

```yaml
apiVersion: networking.istio.io/v1
kind: ServiceEntry
metadata:
  name: <destination.cluster if set, else host with dots replaced by dashes>
  namespace: {{ .Release.Namespace }}
  labels:
    <labels from the RouteConfiguration, see labels.md>
spec:
  hosts:
  - <hostname from endpoint>
  location: MESH_EXTERNAL
  resolution: DNS
  ports:
  - number: <parsed port>
    name: https
    protocol: HTTPS
```

Port defaults: `https://` → 443 with `name: https` / `protocol: HTTPS`; `http://` with no port →
80 with `name: http` / `protocol: HTTP`; explicit `:port` wins. `destination.cluster` is used
only as the ServiceEntry (and often DestinationRule) **name**; `spec.hosts` is always the
endpoint hostname (`github.com`, not `github`).

Do not set `istio.io/use-waypoint` — that would switch the call path to host-based egress.

---

### Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <TlsDef.spec.name>
  namespace: {{ .Release.Namespace }}
  labels:
    <labels from the TlsDef if present, else from the consuming RouteConfiguration>
type: Opaque
stringData:
  ca.crt: |
    <tls.trustedCA verbatim, including Helm expressions>
  # MUTUAL only:
  tls.crt: |
    <tls.clientCert>
  tls.key: |
    <tls.privateKey>
```

Use `stringData` so PEMs and `{{ .Values.* }}` are not base64-wrapped. Do not edit any Secret
that this skill did not generate from a TlsDef.

---

### HTTPRoute changes for egress external destinations

Applied in [route-configuration-mapping.md](route-configuration-mapping.md) when the
destination matches the egress-external rule:

1. `parentRefs` stay Gateway (`egress-gateway`): path-based front door.
2. `backendRefs` use Istio's Hostname backend (the ServiceEntry host), not a Kubernetes
   Service:

```yaml
  backendRefs:
  - group: networking.istio.io
    kind: Hostname
    name: <hostname from endpoint>
    port: <parsed port>
    weight: 1
```

3. Set `URLRewrite` `hostname` to that same host even when the source has no `hostRewrite`
   (Core Mesh uses the cluster endpoint as upstream authority). Merge with `prefixRewrite`
   in one filter when both apply.
4. `hosts: ["*"]` on an egress virtualService: omit `spec.hostnames` (same as other
   gateways). Do **not** raise the east-west `*` MANUAL REVIEW.

---

### Canonical example (cluster-level TlsDef)

Source:

```yaml
apiVersion: nc.core.mesh/v3
kind: TlsDef
spec:
  name: custom-cert
  tls:
    enabled: true
    insecure: false
    sni: github.com
    trustedCA: |
      {{ .Values.EGRESS_TRUSTED_CA }}
---
apiVersion: nc.core.mesh/v3
kind: RouteConfiguration
metadata:
  name: echo-route
spec:
  gateways:
    - egress-gateway
  virtualServices:
    - name: egress-gateway
      hosts: ["*"]
      removeHeaders: ["Origin", "Authorization"]
      routeConfiguration:
        routes:
          - destination:
              cluster: github
              endpoint: https://github.com
              tlsConfigName: custom-cert
            rules:
              - match:
                  prefix: /github
                  headerMatchers:
                    - name: tenant-id
                      exactMatch: cloud-common
                prefixRewrite: /
```

Istio output (same `-istio` sibling as the route, Secret may sit in the TlsDef sibling):

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: echo-route
spec:
  parentRefs:
  - group: gateway.networking.k8s.io
    kind: Gateway
    name: egress-gateway
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /github
      headers:
      - name: tenant-id
        value: cloud-common
    filters:
    - type: RequestHeaderModifier
      requestHeaderModifier:
        remove:
        - Origin
        - Authorization
    - type: URLRewrite
      urlRewrite:
        hostname: github.com
        path:
          type: ReplacePrefixMatch
          replacePrefixMatch: /
    backendRefs:
    - group: networking.istio.io
      kind: Hostname
      name: github.com
      port: 443
      weight: 1
---
apiVersion: networking.istio.io/v1
kind: ServiceEntry
metadata:
  name: github
  namespace: {{ .Release.Namespace }}
spec:
  hosts:
  - github.com
  location: MESH_EXTERNAL
  resolution: DNS
  ports:
  - number: 443
    name: https
    protocol: HTTPS
---
apiVersion: v1
kind: Secret
metadata:
  name: custom-cert
  namespace: {{ .Release.Namespace }}
type: Opaque
stringData:
  ca.crt: |
    {{ .Values.EGRESS_TRUSTED_CA }}
---
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: custom-cert
  namespace: {{ .Release.Namespace }}
spec:
  host: github.com
  trafficPolicy:
    tls:
      mode: SIMPLE
      credentialName: custom-cert
      sni: github.com
```

---

### Gateway-level TlsDef (axis of variation)

`trustedForGateways: [egress-gateway]`, no `sni`, no `tlsConfigName` on the route. Emit one
Secret named `spec.name`. For each egress external host that has no cluster-level profile,
emit a DestinationRule `spec.host: <that host>`, `credentialName: <spec.name>`,
`sni: <that host>`. HTTPRoute and ServiceEntry follow the same templates as above.

---

### mTLS (axis of variation)

When `clientCert` and `privateKey` are both set, the DestinationRule uses `mode: MUTUAL` and
the Secret also has `tls.crt` / `tls.key`. Everything else is unchanged.

---

### Fields that MUST be flagged with `# ⚠ MANUAL REVIEW`

| Source | Trigger |
|---|---|
| `RouteDestination.tlsConfigName` | no matching TlsDef in the chart |
| `https` egress destination | no TlsDef at any priority (system-CA fallback) |
| `TlsDef.spec.tls` | empty `trustedCA` while `insecure: false` |
| `TlsDef.spec.tls` | only one of `clientCert` / `privateKey` |
| `TlsDef.spec.trustedForGateways` | any value other than `egress-gateway` |
| Gateway-level `TlsDef.spec.tls.sni` | set (illegal in Core Mesh; ignored) |
| Two TlsDefs | same `spec.name`, different level (cluster vs gateway) |
| `TlsDef.spec.overridden` | `true` |
| `TlsDef` | no consuming egress destination |
| `RouteConfiguration.spec.gateways` | mix of egress and ingress/mesh |
| `RouteDestination.tlsEndpoint` | non-empty on an egress route |
| DestinationRule | TLS origination vs another policy on the same `spec.host` |

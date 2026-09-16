# Egress Routing

How a service reaches an address outside the cluster through `egress-gateway`, on Istio Ambient Mesh
and on Cloud-Core Mesh, and where the two behave differently.

The gateway itself is covered in [Istio for Developers](./development/istio-for-devs.md#egress-gateway);
this page covers the routes and their TLS.

## Table of Contents

- [The request path](#the-request-path)
- [Resources](#resources)
- [TLS origination](#tls-origination)
- [Secrets and RBAC](#secrets-and-rbac)
- [Where the meshes differ](#where-the-meshes-differ)
- [Troubleshooting](#troubleshooting)

## The request path

Egress is **path-based**. A client calls the gateway over plain HTTP with a path prefix; the gateway
opens the outbound connection, originating TLS if the route says so. Clients never address the
external host directly.

```text
app ──HTTP──> egress-gateway:8080 /egress/github/...
                     │  match prefix, strip it, rewrite the authority
                     └──HTTPS──> github.com:443      TLS originated here
```

Two consequences worth knowing up front: the client needs no certificates, and an external host is
reachable only through a route somebody declared for it.

## Resources

| Cloud-Core Mesh | Istio | Purpose |
|---|---|---|
| `RouteConfiguration` on `egress-gateway` | `HTTPRoute` with `parentRefs` → the Gateway | the path-based front door |
| `RouteDestination.endpoint` | `ServiceEntry` (`MESH_EXTERNAL`, `resolution: DNS`) + `backendRef` `kind: Hostname` | makes the external host routable |
| `TlsDef` | `Secret` + `DestinationRule` `trafficPolicy.tls` | originates TLS to that host |

`egress-gateway` is a **platform gateway**, created by
[qubership-core-mesh-config](https://github.com/Netcracker/qubership-core-mesh-config). An
application chart references it by name and must not define a `Gateway` of its own — a second object
of the same name collides with the platform's.

```yaml
# one rule of an HTTPRoute attached to egress-gateway
- matches:
  - path:
      type: PathPrefix
      value: /egress/github
  filters:
  - type: URLRewrite
    urlRewrite:
      hostname: github.com          # the authority the external host sees
      path:
        type: ReplacePrefixMatch
        replacePrefixMatch: /
  backendRefs:
  - group: networking.istio.io
    kind: Hostname                  # resolved by the ServiceEntry, not a Service
    name: github.com
    port: 443
```

## TLS origination

The `DestinationRule` for the external host decides how the gateway connects:

| Intent | `trafficPolicy.tls` |
|---|---|
| verify against a private CA | `mode: SIMPLE`, `credentialName: <secret>` |
| verify against public CAs | `mode: SIMPLE`, no `credentialName` |
| skip verification | `mode: SIMPLE`, `insecureSkipVerify: true` |
| present a client certificate | `mode: MUTUAL`, `credentialName: <secret>` |

Istio allows **one `DestinationRule` per host**, so two destinations needing different TLS settings
need different hostnames. In Cloud-Core Mesh the equivalent profiles are `TlsDef` objects resolved in
priority order: a `tlsConfigName` on the route, then a gateway-level `TlsDef`
(`trustedForGateways: [egress-gateway]`), then a generated `<cluster>-tls`.

## Secrets and RBAC

A `credentialName` Secret lives in the **egress gateway's namespace**, and two details decide whether
it works:

- **Type.** A CA-only Secret is `Opaque` with `ca.crt`. A Secret carrying a client certificate must be
  `kubernetes.io/tls`; from an `Opaque` one Istio resolves `ca.crt` and silently ignores
  `tls.crt` / `tls.key`. `type` is immutable, so changing it means deleting the Secret first.
- **Permission.** istiod pushes the Secret over SDS only after checking that the gateway's
  ServiceAccount may read Secrets in the namespace. That check is a SubjectAccessReview carrying **no
  resource name**, so a `Role` narrowed with `resourceNames` never satisfies it. The grant ships with
  the gateway in `qubership-core-mesh-config`.

Neither failure is reported. The `DestinationRule` renders, the config dump looks correct, and the
connection fails with `remote connection failure` and nothing in either log.

## Where the meshes differ

Three behaviours change on migration, none of them announced by a warning.

**SNI.** Cloud-Core Mesh sends SNI only when the `TlsDef` sets `tls.sni`; a gateway-level profile is
forbidden from setting it, and the endpoint address is used only when the control plane runs with
`SNI_PROPAGATION_ENABLED=true`, which defaults to `false`. Istio's `DestinationRule` always carries an
`sni`. A virtual-hosted endpoint may therefore serve a different certificate after migration.

**The authority.** Both meshes rewrite it to the endpoint host — Cloud-Core Mesh gives every route it
creates the cluster endpoint as its authority. Cloud-Core Mesh includes the port, and Gateway API's
`URLRewrite.hostname` cannot, so an upstream reading `Host` verbatim sees `host:port` before migration
and a bare host after.

**An unmatched path.** Cloud-Core Mesh has nothing to match and returns `404`. The Istio egress
gateway carries a catch-all route to `egress-fallback-service`, so an unmatched path reaches that
service instead. A route whose match is narrowed by a header will not fall through to a `404`.

## Troubleshooting

| Symptom | Look at |
|---|---|
| `503 no healthy upstream` | the host does not resolve — check the `ServiceEntry` and cluster DNS |
| `503 ... remote connection failure` | TLS origination failed — Secret `type`, the RBAC grant, or the CA |
| `421` from the external host | the gateway offered an SNI that host does not serve |
| reached the wrong backend | rule order — `HTTPRoute` rules are matched by path specificity, not file order |

`istioctl proxy-config secret deploy/egress-gateway-istio -n <ns>` shows which credentials actually
reached the gateway; a `credentialName` missing from that list is the RBAC or type problem above.

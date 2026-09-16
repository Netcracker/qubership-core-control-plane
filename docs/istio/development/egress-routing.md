# Egress Routing

How to route traffic from a service to an address outside the cluster through `egress-gateway`, and
how to originate TLS to that address.

## Table of Contents

- [How egress works](#how-egress-works)
- [Plain HTTP](#plain-http)
- [HTTPS with a public CA](#https-with-a-public-ca)
- [HTTPS with a private CA](#https-with-a-private-ca)
- [Mutual TLS](#mutual-tls)
- [Skipping verification](#skipping-verification)
- [Secrets](#secrets)
- [Rules that apply to every case](#rules-that-apply-to-every-case)
- [Troubleshooting](#troubleshooting)

## How egress works

Egress is **path-based**. A client calls the gateway over plain HTTP with a path prefix; the gateway
strips the prefix, rewrites the authority and opens the outbound connection, originating TLS when the
route says so. Clients never address the external host and never hold certificates.

```text
app ──HTTP──> egress-gateway:8080 /egress/github/repos
                     │  match prefix, strip it, rewrite the authority
                     └──HTTPS──> github.com:443      TLS originated here
```

Every case below needs the same two objects, plus a `DestinationRule` when TLS is involved:

- a `ServiceEntry`, which makes the external host routable from inside the mesh
- an `HTTPRoute` attached to `egress-gateway`, which maps a path prefix onto that host

`egress-gateway` is created by
[qubership-core-mesh-config](https://github.com/Netcracker/qubership-core-mesh-config). Reference it
by name; do not declare a `Gateway` of your own, as a second object of that name collides with the
platform's.

## Plain HTTP

No TLS, so no `DestinationRule`. Note `protocol: HTTP` and the matching port.

```yaml
apiVersion: networking.istio.io/v1
kind: ServiceEntry
metadata:
  name: legacy-api
spec:
  hosts:
    - legacy.example.com
  location: MESH_EXTERNAL
  resolution: DNS
  ports:
    - number: 80
      name: http
      protocol: HTTP
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: legacy-api-egress
spec:
  parentRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: egress-gateway
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /egress/legacy
      filters:
        - type: URLRewrite
          urlRewrite:
            hostname: legacy.example.com
            path:
              type: ReplacePrefixMatch
              replacePrefixMatch: /
      backendRefs:
        - group: networking.istio.io
          kind: Hostname
          name: legacy.example.com
          port: 80
          weight: 1
```

A client then reaches `http://legacy.example.com/orders` by calling
`http://egress-gateway:8080/egress/legacy/orders`.

## HTTPS with a public CA

The host presents a certificate signed by a well-known CA. Declare the port as `HTTPS` and add a
`DestinationRule` with `mode: SIMPLE` and no `credentialName`, so Istio validates against its system
CA bundle.

```yaml
apiVersion: networking.istio.io/v1
kind: ServiceEntry
metadata:
  name: github
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
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: github
spec:
  host: github.com
  trafficPolicy:
    tls:
      mode: SIMPLE
      sni: github.com
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: github-egress
spec:
  parentRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: egress-gateway
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /egress/github
      filters:
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
```

## HTTPS with a private CA

The host presents a certificate signed by a CA of your own. Put the CA in an `Opaque` Secret under
`ca.crt` and name it from the `DestinationRule`. The `ServiceEntry` and `HTTPRoute` are as above.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: partner-ca
type: Opaque
stringData:
  ca.crt: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
---
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: partner
spec:
  host: partner.example.com
  trafficPolicy:
    tls:
      mode: SIMPLE
      credentialName: partner-ca
      sni: partner.example.com
```

## Mutual TLS

The host demands a client certificate. The Secret carries the client keypair **and** the CA used to
verify the host, and its type must be `kubernetes.io/tls`.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: partner-mtls
type: kubernetes.io/tls      # not Opaque — see Secrets below
stringData:
  tls.crt: |                 # client certificate the gateway presents
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
  tls.key: |                 # its private key
    -----BEGIN PRIVATE KEY-----
    ...
    -----END PRIVATE KEY-----
  ca.crt: |                  # CA used to verify the host
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
---
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: partner-mtls
spec:
  host: partner.example.com
  trafficPolicy:
    tls:
      mode: MUTUAL
      credentialName: partner-mtls
      sni: partner.example.com
```

Mutual TLS additionally needs the gateway's ServiceAccount to be allowed to read Secrets — see
[Secrets](#secrets).

## Skipping verification

For a host with a self-signed or otherwise unverifiable certificate. TLS is still negotiated; the
certificate is simply not checked.

```yaml
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: staging-partner
spec:
  host: staging.partner.example.com
  trafficPolicy:
    tls:
      mode: SIMPLE
      insecureSkipVerify: true
      sni: staging.partner.example.com
```

Set `sni` even here. Without it the gateway offers no server name, and a virtual-hosted endpoint
answers with whatever certificate it serves by default — which is rarely the one you meant to reach.

## Secrets

A Secret named by `credentialName` must live in the **egress gateway's namespace**, and two details
decide whether it is used at all.

**Type.** `Opaque` is correct for a CA-only Secret. A Secret carrying a client certificate must be
`kubernetes.io/tls`; from an `Opaque` one Istio resolves `ca.crt` and silently ignores `tls.crt` and
`tls.key`, so mutual TLS fails while the configuration looks right. `type` is immutable — changing it
means deleting the Secret and letting it be recreated.

**Permission.** istiod pushes a Secret over SDS only after checking that the gateway's ServiceAccount
may read Secrets in that namespace. The check is a SubjectAccessReview carrying **no resource name**,
so a `Role` narrowed with `resourceNames` never satisfies it even though
`kubectl auth can-i get secrets/<name>` answers `yes`. The grant ships with the gateway in
`qubership-core-mesh-config`.

Neither failure is announced. The `DestinationRule` renders, the Envoy config dump shows the SDS
reference, and the connection fails with `remote connection failure` and nothing in either log.

## Rules that apply to every case

- **One `DestinationRule` per host.** Two destinations needing different TLS settings need different
  hostnames.
- **`ServiceEntry.spec.hosts` is the real hostname**, and the `backendRef` name must match it exactly.
- **Always set `sni`** on a TLS `DestinationRule`, including with `insecureSkipVerify`.
- **Set `URLRewrite.hostname`.** Without it the external host receives the gateway's authority, which
  a virtual-hosted endpoint will not recognize.
- **Rules are matched by path specificity, not file order.** Put header-matched and longer-prefix
  rules first if you rely on ordering.
- **The external host must resolve** from inside the cluster, since `resolution: DNS` makes the
  gateway resolve it.

## Troubleshooting

| Symptom | Look at |
|---|---|
| `503 no healthy upstream` | the host does not resolve — check the `ServiceEntry` and cluster DNS |
| `503 ... remote connection failure` | TLS origination failed — Secret `type`, the RBAC grant, or the CA |
| `421` from the external host | the gateway offered an SNI that host does not serve |
| reached the wrong backend | rule order — rules are matched by path specificity |
| plain-text bytes at a TLS port | no `DestinationRule` for that host, or `protocol: HTTP` on an HTTPS port |

`istioctl proxy-config secret deploy/egress-gateway-istio -n <ns>` lists the credentials that
actually reached the gateway. A `credentialName` missing from that list is the Secret type or the
RBAC grant.

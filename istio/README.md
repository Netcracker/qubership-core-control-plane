### Test scenario
- Deploy cloud core and istio core components to correspanding namespaces
- Deploy and register test service
```
    kubectl apply -f test_service.yaml
```
- Run test kubernetes job cloud core with no istio enabled for workspace
```
    kubectl apply -f test_job.yaml
```
- Label namespace to work with istio
```
    kubectl label namespace cloud-core istio.io/dataplane-mode=ambient
```
- Run test kubernetes job cloud core with istio enabled for workspace
- Label namespace to work with istio waypoint
```
    kubectl label ns cloud-core istio.io/use-waypoint=waypoint
```
- Run test kubernetes job cloud core with istio and its waypoint enabled for workspace



### Issues:
There is an issue with requests to the istio gateway from a neighboring node.

In Ambient Mesh, data flow looks like this:

Client pod
  ↓ (redirected by ztunnel via eBPF)
Source node's ztunnel
  ↓ (mTLS HBONE tunnel)
Destination node's ztunnel
  ↓
Gateway workload (Envoy proxy)

When both pods are on the same node:

client → ztunnel(local) → gateway(pod)
the traffic doesn’t need inter-node mTLS.
But when they’re on different nodes:

client → ztunnel(src node) → ztunnel(dst node) → gateway
the HBONE connection requires a valid SPIFFE identity to establish mTLS between the ztunnels.


Test pod (with hey) doesn’t have a src.identity in logs:

src.identity=nil
error="io error: deadline has elapsed"

So when ztunnel A (source node) tries to open an HBONE mTLS session to ztunnel B (gateway’s node),
the handshake fails — there’s no certificate for the client, so ztunnel rejects the connection.

When both are local, ztunnel just proxies traffic internally over TCP, skipping HBONE, so it works.

Resolves temporary by affinity rules. Also there is a WA to allow plaintext so ztunnel forwards without HBONE:
```
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: gateway-permissive
  namespace: cloud-core
spec:
  selector:
    matchLabels:
      gateway.networking.k8s.io/gateway-name: istio-gateway
  mtls:
    mode: PERMISSIVE

```
Permamnent fix should be provided by istio community in 1.28.0 version. Starting in Istio 1.28, ztunnel assigns SPIFFE identities to all pods with ambient redirection, even pure clients, also configurable connection timeouts will be supported.

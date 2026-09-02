# Managing HTTPRoutes at runtime

Your service can create and change its own `HTTPRoute` resources through the Kubernetes API. `HTTPRoute` is a
custom resource in the `gateway.networking.k8s.io` group, so it goes through the same API server and the same
RBAC as any built-in resource.

You need three things: a ServiceAccount, a Role bound to it, and that ServiceAccount set on your Deployment.
Then your code talks to the API using the token Kubernetes mounts into the pod.

## 1. Grant permissions

Create a ServiceAccount, a Role, and a RoleBinding in the namespace where your routes live.

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: route-writer
  namespace: <your-namespace>
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: httproute-writer
  namespace: <your-namespace>
rules:
  - apiGroups:
      - gateway.networking.k8s.io
    resources:
      - httproutes
    verbs:
      - get
      - list
      - watch
      - create
      - update
      - patch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: httproute-writer
  namespace: <your-namespace>
subjects:
  - kind: ServiceAccount
    name: route-writer
    namespace: <your-namespace>
roleRef:
  kind: Role
  name: httproute-writer
  apiGroup: rbac.authorization.k8s.io
```

A Role grants access in one namespace. To manage routes in several namespaces, use a ClusterRole with a
RoleBinding in each target namespace.

## 2. Set `serviceAccountName` in your Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: <your-service>
  namespace: <your-namespace>
spec:
  template:
    spec:
      serviceAccountName: route-writer
      containers:
        - name: app
          image: <your-image>
```

Without `serviceAccountName`, the pod runs as the `default` ServiceAccount of its namespace. The token is still
mounted and authentication still succeeds, so the failure surfaces as `403 Forbidden` on the write, not as an
authentication error. A missing `serviceAccountName` is the most common cause.

## If the route has no effect

A route can be stored successfully and still carry no traffic. Read the `Accepted` and `ResolvedRefs` conditions
under `status.parents` on the route: the Gateway controller fills them in, and their `reason` names what it
rejected.

If the Gateway or the backend Service lives in another namespace, permission to reference it is granted by the
owner of that namespace, not by your route.

## Further reading

- [Gateway API reference](https://gateway-api.sigs.k8s.io/references/spec/) — the full schema of `HTTPRoute` and
  every field it accepts.
- [Cross-namespace routing](https://gateway-api.sigs.k8s.io/guides/multiple-ns/) — attaching routes to a Gateway
  in another namespace.
- [Server-side apply](https://kubernetes.io/docs/reference/using-api/server-side-apply/) — writing a resource in
  one idempotent call instead of a read-modify-write cycle.

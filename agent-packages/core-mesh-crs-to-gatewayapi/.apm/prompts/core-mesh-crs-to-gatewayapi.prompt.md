---
description: Convert Cloud-Core Mesh Helm CRs to Istio Gateway API resources in a chart.
argument-hint: <path to Helm chart / templates folder>
---

Apply the `core-mesh-crs-to-gatewayapi` skill to convert Cloud-Core Mesh custom
resources to Istio Gateway API resources.

Chart / templates path: $ARGUMENTS

If no path was given above, ask which Helm chart or templates folder to convert.

Follow the skill exactly:

1. Discover files containing `FacadeService`, `Gateway`, `RouteConfiguration`
   (or `Mesh`) CRs — only these are in scope.
2. Wrap originals in `{{- if eq .Values.SERVICE_MESH_TYPE "Core" }}` guards.
3. Generate `-istio.yaml` siblings guarded by `SERVICE_MESH_TYPE "Istio"`,
   converting Gateway(ingress/egress) → Istio Gateway and RouteConfiguration →
   HTTPRoute, omitting FacadeService and mesh-type Gateways (east-west HTTPRoutes
   with Service parentRefs instead).
4. Resolve gateway types — ask the user about any unresolved gateway before
   generating its routes.
5. Add `SERVICE_MESH_TYPE` to `values.yaml` / `values.schema.json`, report the
   detected backend reference and output labels, and flag any
   `⚠ MANUAL REVIEW REQUIRED` fields.

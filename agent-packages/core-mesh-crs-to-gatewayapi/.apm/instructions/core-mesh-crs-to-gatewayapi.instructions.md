---
description: Converting Cloud-Core Mesh Helm CRs to Istio Gateway API resources.
applyTo: "**/*.{yaml,yml}"
---

When working with Qubership Helm templates that contain Cloud-Core Mesh custom
resources (`FacadeService`, `Gateway`, `RouteConfiguration`, or `Mesh` CRs) and
the user asks to migrate, convert, or transform them to Istio / Gateway API,
apply the `core-mesh-crs-to-gatewayapi` skill.

It wraps the originals in `SERVICE_MESH_TYPE=Core` guards, generates `-istio.yaml`
siblings guarded by `SERVICE_MESH_TYPE=Istio`, converts Gateway/RouteConfiguration
to Istio `Gateway` + `HTTPRoute`, and updates `values.yaml` / `values.schema.json`.
Touch only mesh-CR files — leave Deployments, Services, `*.tpl` helpers, and other
kinds unchanged.

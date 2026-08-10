---
description: Converting Cloud-Core Mesh Helm CRs to Istio Ambient Mesh resources.
applyTo: "**/*.{yaml,yml}"
---

When editing Helm templates (`*.yaml` / `*.yml`) that contain Cloud-Core Mesh
custom resources (`FacadeService`, `Gateway`, `RouteConfiguration`, `Mesh`,
`StatefulSession`, `LoadBalance`, `HttpFilters`) and asked to migrate, convert,
or transform them to Istio / Gateway API, apply the `core-mesh-crs-to-istio`
skill.

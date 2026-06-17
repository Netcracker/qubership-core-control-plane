---
description: Running the full Cloud-Core Mesh to Istio migration on a service or Helm chart.
applyTo: "**/*.{yaml,yml,go,java,xml,mod}"
---

When the user asks to migrate a service from Cloud-Core Mesh to Istio (or Istio
Ambient Mesh), run the whole migration guide end-to-end, or runs
`/core-mesh-to-istio-migration`, apply the `core-mesh-to-istio-migration` skill.

It orchestrates the two atomic skills (`core-mesh-crs-to-gatewayapi` and
`httproute-from-code`), performs the remaining steps (mesh-aware libraries,
`SERVICE_MESH_TYPE`, Maven plugin, Istio guards, duplicate-rule detection), and
maintains `MIGRATION_LOG.md`. Do not migrate by hand — drive it through the skill.

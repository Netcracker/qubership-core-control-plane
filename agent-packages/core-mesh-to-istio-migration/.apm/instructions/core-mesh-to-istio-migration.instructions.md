---
description: Running the full Cloud-Core Mesh to Istio migration on a service or Helm chart.
applyTo: "**/*.{yaml,yml,go,java,xml,mod}"
---

When asked to migrate a service from Cloud-Core Mesh to Istio (or Istio Ambient
Mesh) or to run the migration guide end-to-end, apply the
`core-mesh-to-istio-migration` skill. It orchestrates the atomic skills, performs
the remaining steps, and maintains `MIGRATION_LOG.md` — do not migrate by hand.

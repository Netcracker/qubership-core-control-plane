---
description: Run the full Cloud-Core Mesh to Istio migration on a service / Helm chart end-to-end.
argument-hint: <path to Helm chart or service directory>
---

Run the `core-mesh-to-istio-migration` skill to migrate a service from
Cloud-Core Mesh to Istio.

Migration target: $ARGUMENTS

If no path was given above, ask for the chart path, the source-code path, and
the service language(s) (Go, Java, or both) before starting.

Follow the skill exactly:

1. Resolve inputs up front (chart path, source-code path, language, build system).
2. Step 1 — invoke `core-mesh-crs-to-gatewayapi` to convert declarative mesh CRs,
   and capture the detected `backendRefName` / `backendRefPort` and output labels.
3. Step 2 — switch to mesh-aware route-registration libraries, set
   `SERVICE_MESH_TYPE`, add the Maven plugin (Java only), and invoke
   `httproute-from-code` to generate HTTPRoutes from code, reusing the captured
   backend reference and labels.
4. Verify every HTTPRoute is wrapped in the Istio guard and scan for duplicate
   rules.
5. Maintain `MIGRATION_LOG.md` after every step and finish with the Final report,
   highlighting all Needs-review items before a PR is raised.

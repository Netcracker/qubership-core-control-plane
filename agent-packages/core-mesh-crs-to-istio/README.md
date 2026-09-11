# core-mesh-crs-to-istio

An APM package that converts Qubership Cloud-Core Mesh custom resources in a
Helm chart (`FacadeService`, `Gateway`, `RouteConfiguration`, `TlsDef`,
`StatefulSession`, `LoadBalance`, `HttpFilters`) to Istio Ambient Mesh resources
— Gateway API `Gateway` + `HTTPRoute`, `DestinationRule`, `ServiceEntry`, and
`TrafficExtension` — in a single pass, while keeping the chart deployable on
**both** mesh types.

## Install

```sh
apm install Netcracker/qubership-core-control-plane/agent-packages/core-mesh-crs-to-istio --target claude
```

This deploys the package's primitives into the consuming repo
(`.claude/skills/`, `.claude/rules/`, and the merged `CLAUDE.md`). Re-run it to
pick up a new version.

## What you get

- The [`SKILL.md`](.apm/skills/core-mesh-crs-to-istio/SKILL.md) — the
  step-by-step transformation, plus its co-located reference files, one per CR
  kind:
  - [`facade-service-mapping.md`](.apm/skills/core-mesh-crs-to-istio/facade-service-mapping.md)
  - [`gateway-mapping.md`](.apm/skills/core-mesh-crs-to-istio/gateway-mapping.md)
  - [`route-configuration-mapping.md`](.apm/skills/core-mesh-crs-to-istio/route-configuration-mapping.md)
  - [`tls-def-mapping.md`](.apm/skills/core-mesh-crs-to-istio/tls-def-mapping.md)
  - [`stateful-session-rule-mapping.md`](.apm/skills/core-mesh-crs-to-istio/stateful-session-rule-mapping.md)
  - [`stateful-session-mapping.md`](.apm/skills/core-mesh-crs-to-istio/stateful-session-mapping.md)
  - [`load-balance-mapping.md`](.apm/skills/core-mesh-crs-to-istio/load-balance-mapping.md)
  - [`lua-filter-mapping.md`](.apm/skills/core-mesh-crs-to-istio/lua-filter-mapping.md)
  - [`labels.md`](.apm/skills/core-mesh-crs-to-istio/labels.md)
- The shared rule-sorting procedure lives in its own package,
  [`path-specificity-sorting`](../path-specificity-sorting) (declared as a
  dependency), referenced as a sibling skill once installed.
- An instruction that fires when you work on Helm templates containing mesh CRs,
  steering the agent to the skill.
- E2E test fixtures under
  [`tests/`](.apm/skills/core-mesh-crs-to-istio/tests) with input, expected
  output, and run instructions.

## Usage

The instruction triggers the skill whenever you ask the agent to migrate or
convert mesh CRs while working on a Helm chart. You can also invoke the skill by
name against a chart or templates folder, e.g. "run core-mesh-crs-to-istio on
helm-templates/my-service", or let the
[`core-mesh-to-istio-migration`](../core-mesh-to-istio-migration) orchestrator
call it as a sub-skill.

It wraps the originals in `SERVICE_MESH_TYPE=Core` guards, generates
`-istio.yaml` siblings guarded by `SERVICE_MESH_TYPE=Istio`, updates
`values.yaml` / `values.schema.json`, and reports the detected backend
reference, output labels, and any `⚠ MANUAL REVIEW` items.

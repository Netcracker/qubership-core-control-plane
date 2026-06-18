# core-mesh-crs-to-gatewayapi

An APM package that converts Qubership Cloud-Core Mesh custom resources
(`FacadeService`, `Gateway`, `RouteConfiguration`) in a Helm chart to Istio
Ambient Mesh **Gateway API** resources (`Gateway` + `HTTPRoute`), while keeping
the chart deployable on **both** mesh types.

## Install

```sh
apm install Netcracker/qubership-core-control-plane/agent-packages/core-mesh-crs-to-gatewayapi --target claude
```

This deploys the package's primitives into the consuming repo
(`.claude/skills/`, `.claude/rules/`, and the merged `CLAUDE.md`). Re-run it to
pick up a new version.

## What you get

- The [`SKILL.md`](.apm/skills/core-mesh-crs-to-gatewayapi/SKILL.md) — the
  step-by-step transformation, plus its co-located reference files:
  - [`facade-service-mapping.md`](.apm/skills/core-mesh-crs-to-gatewayapi/facade-service-mapping.md)
  - [`gateway-mapping.md`](.apm/skills/core-mesh-crs-to-gatewayapi/gateway-mapping.md)
  - [`route-configuration-mapping.md`](.apm/skills/core-mesh-crs-to-gatewayapi/route-configuration-mapping.md)
  - [`labels.md`](.apm/skills/core-mesh-crs-to-gatewayapi/labels.md)
- The shared rule-sorting procedure lives in its own package,
  [`path-specificity-sorting`](../path-specificity-sorting) (declared as a
  dependency), referenced as a sibling skill once installed.
- An instruction that fires when you work on Helm templates containing mesh CRs,
  steering the agent to the skill.

## Usage

The instruction triggers the skill whenever you ask the agent to migrate or
convert mesh CRs while working on a Helm chart. You can also invoke the skill by
name against a chart or templates folder, e.g. "run core-mesh-crs-to-gatewayapi
on helm-templates/my-service".

It wraps the originals in `SERVICE_MESH_TYPE=Core` guards, generates
`-istio.yaml` siblings guarded by `SERVICE_MESH_TYPE=Istio`, updates
`values.yaml` / `values.schema.json`, and reports the detected backend reference,
output labels, and any `⚠ MANUAL REVIEW REQUIRED` fields.

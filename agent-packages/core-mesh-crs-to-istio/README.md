# core-mesh-crs-to-istio

An APM package that converts standalone Qubership Cloud-Core Mesh traffic-policy
custom resources (`StatefulSession`, `LoadBalance`) in a Helm chart to Istio
`DestinationRule` resources, while keeping the chart deployable on **both** mesh
types.

`StatefulSession` embedded inside a `RouteConfiguration` rule is out of scope
here — it is handled by
[`core-mesh-crs-to-gatewayapi`](../core-mesh-crs-to-gatewayapi) as part of
HTTPRoute generation.

## Install

```sh
apm install Netcracker/qubership-core-control-plane/agent-packages/core-mesh-crs-to-istio --target claude
```

This deploys the package's primitives into the consuming repo. Re-run it to pick
up a new version.

## What you get

- The [`SKILL.md`](.apm/skills/core-mesh-crs-to-istio/SKILL.md) — the
  step-by-step transformation, plus its co-located reference files:
  - [`stateful-session-mapping.md`](.apm/skills/core-mesh-crs-to-istio/stateful-session-mapping.md)
    — StatefulSession → DestinationRule (`consistentHash.httpCookie`)
  - [`load-balance-mapping.md`](.apm/skills/core-mesh-crs-to-istio/load-balance-mapping.md)
    — LoadBalance → DestinationRule (`consistentHash.*`)
- An e2e test fixture under
  [`tests/`](.apm/skills/core-mesh-crs-to-istio/tests) with input, expected
  output, and run instructions.

## Usage

Invoke the skill by name against a chart or templates folder, e.g. "run
core-mesh-crs-to-istio on helm-templates/my-service", or let the
[`core-mesh-to-istio-migration`](../core-mesh-to-istio-migration) orchestrator
call it as a sub-skill.

It wraps the originals in `SERVICE_MESH_TYPE=Core` guards, generates `-istio.yaml`
siblings guarded by `SERVICE_MESH_TYPE=Istio`, and reports any
`⚠ MANUAL REVIEW` items (endpoint-level targeting, multi-policy LoadBalance,
overridden CRs).

# mesh-build-wiring

An APM package that wires a service's build and deployment for mesh-type
awareness: switches to mesh-aware route-registration libraries (Java Spring /
Quarkus, Go), sets the `SERVICE_MESH_TYPE` environment variable in Helm values
and Deployments, and adds the `httproutes-generator-maven-plugin` for Java
services.

Extracted from the `core-mesh-to-istio-migration` orchestrator (its former
Steps 2.1–2.3) so it can run standalone or in a sub-agent with its own context.

## Install

```sh
apm install Netcracker/qubership-core-control-plane/agent-packages/mesh-build-wiring --target claude
```

## What you get

- The [`SKILL.md`](.apm/skills/mesh-build-wiring/SKILL.md) — dependency
  migration rules per language/framework, the `SERVICE_MESH_TYPE` wiring table,
  the Maven plugin configuration, and a contract (typed inputs, report file)
  for orchestrated or standalone execution.

## Usage

Invoke the skill by name against a service repo, e.g. "run mesh-build-wiring on
.", or let the [`core-mesh-to-istio-migration`](../core-mesh-to-istio-migration)
orchestrator call it as a sub-skill with resolved inputs.

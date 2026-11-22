# mesh-build-wiring

An APM package that wires a service's build and deployment for mesh-type
awareness: switches to mesh-aware route-registration libraries (Java Spring /
Quarkus, Go), sets the `SERVICE_MESH_TYPE` environment variable in Helm values
and Deployments, and adds the right Maven plugin for Java services —
`httproutes-generator-spring-maven-plugin` (Spring) or
`httproutes-generator-maven-plugin` (Quarkus), both at `>= 1.2.1`.

Extracted from the `core-mesh-to-istio-migration` orchestrator (its former
Steps 2.1–2.3) so it can run standalone or in a sub-agent with its own context.

## Install

```sh
apm install Netcracker/qubership-core-control-plane/agent-packages/mesh-build-wiring --target claude
```

## What you get

- The [`SKILL.md`](.apm/skills/mesh-build-wiring/SKILL.md) — dependency
  migration rules per language/framework, the `SERVICE_MESH_TYPE` wiring table,
  the Maven plugin configuration (framework detection, which artifact to use,
  explicit `backendRefVal`), and a contract (typed inputs, report file) for
  orchestrated or standalone execution.
- A dependency on [`public-gateway-authz-policy`](../public-gateway-authz-policy)
  — this skill never generates an `AuthorizationPolicy`; that package covers
  the recommended manual-authoring workflow.

## Usage

Invoke the skill by name against a service repo, e.g. "run mesh-build-wiring on
.", or let the [`core-mesh-to-istio-migration`](../core-mesh-to-istio-migration)
orchestrator call it as a sub-skill with resolved inputs.

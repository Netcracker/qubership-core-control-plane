# istio-migration-validate

An APM package that validates the output of a Core Mesh → Istio migration:
every HTTPRoute file is wrapped in the `SERVICE_MESH_TYPE=Istio` guard (missing
guards are added — the one safe automatic fix), no HTTPRoute leaks when the
chart renders in Core mode, and duplicate HTTPRoute rules (same parent + equal
match) across declarative, annotation-generated, and code-generated routes are
flagged for review. It also flags imperative control-plane API calls left in
shell scripts and manifests, which the migration never converts.

Extracted from the `core-mesh-to-istio-migration` orchestrator (its former
Steps 2.5–2.7) so it can also run standalone — for example as a chart check
during PR review.

## Install

```sh
apm install Netcracker/qubership-core-control-plane/agent-packages/istio-migration-validate --target claude
```

## What you get

- The [`SKILL.md`](.apm/skills/istio-migration-validate/SKILL.md) — the guard
  verification, the two `helm template` render checks, the duplicate-rule
  comparison procedure, the imperative control-plane call scan, and a contract
  (typed inputs, report file) for orchestrated or standalone execution.

## Usage

Invoke the skill by name against a chart, e.g. "run istio-migration-validate on
helm-templates/my-service", or let the
[`core-mesh-to-istio-migration`](../core-mesh-to-istio-migration) orchestrator
call it as a sub-skill.

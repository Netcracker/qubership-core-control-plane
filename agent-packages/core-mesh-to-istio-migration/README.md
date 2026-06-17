# core-mesh-to-istio-migration

An APM package that drives the **full** Cloud-Core Mesh → Istio migration
end-to-end. It is an orchestrator: it delegates the heavy lifting to the
[`core-mesh-crs-to-gatewayapi`](../core-mesh-crs-to-gatewayapi) and
[`httproute-from-code`](../httproute-from-code) packages, performs the remaining
steps itself, and keeps an auditable `MIGRATION_LOG.md`.

## Install

```sh
apm install Netcracker/qubership-core-control-plane/agent-packages/core-mesh-to-istio-migration --target claude
```

This pulls in the two atomic sub-skills it delegates to
([`core-mesh-crs-to-gatewayapi`](../core-mesh-crs-to-gatewayapi) and
[`httproute-from-code`](../httproute-from-code)) — and transitively the shared
[`path-specificity-sorting`](../path-specificity-sorting) procedure — as declared
`dependencies`, so they all resolve as siblings under `.claude/skills/`.

This deploys the package's primitives into the consuming repo
(`.claude/skills/`, `.claude/commands/`, `.claude/rules/`, and the merged
`CLAUDE.md`). Re-run it to pick up a new version.

## What you get

- The [`SKILL.md`](.apm/skills/core-mesh-to-istio-migration/SKILL.md) — the full
  migration procedure (Step 1 mesh-CR conversion through Step 2.6 duplicate-rule
  detection), the mandatory `MIGRATION_LOG.md` format, the error policy, and
  idempotent reruns.
- An instruction that fires when you ask to run an Istio migration on a service
  or Helm chart.
- A [`/core-mesh-to-istio-migration`](.apm/prompts/core-mesh-to-istio-migration.prompt.md)
  slash command to run the whole migration on demand.

## Usage

On demand — run the slash command against a chart or service directory:

```text
/core-mesh-to-istio-migration helm-templates/my-service
```

Automatic — the instruction triggers the skill whenever you ask the agent to
migrate a service from Core Mesh to Istio, so no command is required.

Either path runs every step in order, delegates to the two atomic skills,
validates the result, and writes a Done / Skipped / Needs-review log to
`MIGRATION_LOG.md` — review every **Needs review** entry before raising a PR.

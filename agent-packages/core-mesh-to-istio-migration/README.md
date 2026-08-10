# core-mesh-to-istio-migration

An APM package that drives the **full** Cloud-Core Mesh → Istio migration
end-to-end. It is an orchestrator: it delegates the heavy lifting to the
[`core-mesh-crs-to-istio`](../core-mesh-crs-to-istio),
[`mesh-build-wiring`](../mesh-build-wiring),
[`httproute-from-code`](../httproute-from-code), and
[`istio-migration-validate`](../istio-migration-validate) packages, resolves
user questions, and keeps an auditable `MIGRATION_LOG.md`.

## Install

```sh
apm install Netcracker/qubership-core-control-plane/agent-packages/core-mesh-to-istio-migration --target claude
```

This pulls in the four atomic sub-skills it delegates to — and transitively the
shared [`path-specificity-sorting`](../path-specificity-sorting) procedure — as
declared `dependencies`, so they all resolve as siblings under
`.claude/skills/`.

This deploys the package's primitives into the consuming repo
(`.claude/skills/`, `.claude/rules/`, and the merged `CLAUDE.md`). Re-run it to
pick up a new version.

## What you get

- The [`SKILL.md`](.apm/skills/core-mesh-to-istio-migration/SKILL.md) — the full
  migration procedure (Step 1 mesh-CR conversion through Step 2.6 duplicate-rule
  detection), the mandatory `MIGRATION_LOG.md` format, the error policy, and
  idempotent reruns.
- An instruction that fires when you ask to run an Istio migration on a service
  or Helm chart, steering the agent to the skill.

## Usage

The instruction triggers the skill whenever you ask the agent to migrate a
service from Core Mesh to Istio. You can also invoke the skill by name against a
chart or service directory, e.g. "run core-mesh-to-istio-migration on
helm-templates/my-service".

It runs every step in order, delegates to the four atomic skills,
validates the result, and writes a Done / Skipped / Needs-review log to
`MIGRATION_LOG.md` — review every **Needs review** entry before raising a PR.

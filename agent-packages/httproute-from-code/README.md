# httproute-from-code

An APM package that generates Kubernetes Gateway API `HTTPRoute` CRs directly
from Go or Java route-registration code, so code-defined routes are reflected in
the Istio cluster configuration.

## Install

```sh
apm install Netcracker/qubership-core-control-plane/agent-packages/httproute-from-code --target claude
```

This deploys the package's primitives into the consuming repo
(`.claude/skills/`, `.claude/rules/`, and the merged `CLAUDE.md`). Re-run it to
pick up a new version.

## What you get

- The [`SKILL.md`](.apm/skills/httproute-from-code/SKILL.md) — how to detect
  route definitions in Go/Java, map them to HTTPRoute rules, and emit one CR per
  route type.
- The shared `rules[]` ordering procedure lives in its own package,
  [`path-specificity-sorting`](../path-specificity-sorting) (declared as a
  dependency), referenced as a sibling skill once installed.
- An instruction that fires when you work on Go/Java route-registration code,
  steering the agent to the skill.

## Usage

The instruction triggers the skill whenever you ask the agent to generate
HTTPRoutes from source code. You can also invoke the skill by name against a file
or directory, e.g. "run httproute-from-code on internal/routes".

It scans the Go/Java files, extracts the route registrations, groups
them by route type, sorts rules by path specificity, and writes one HTTPRoute CR
per type to `helm-templates/<service>/templates/source-code-httproutes.yaml`.
Pass `backendRefName` / `backendRefPort` / `routeLabels` to keep the generated
routes consistent with the declarative and Maven-plugin output.

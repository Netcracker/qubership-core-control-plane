---
description: Generating Gateway API HTTPRoute CRs from Go/Java route-registration code.
applyTo: "**/*.{go,java}"
---

When the user asks to generate HTTPRoute CRs from source code, convert route
registrations to HTTPRoute YAML, or extract routes from Go/Java files (or runs
`/httproute-from-code`), apply the `httproute-from-code` skill.

It scans Go (`*.go`) and Java (`*.java`) files for route-registration
definitions (`routeregistration.Route` / `RouteEntry`), groups them by route
type, and emits one HTTPRoute CR per type to
`helm-templates/<service>/templates/source-code-httproutes.yaml`. Use the caller's
`backendRefName` / `backendRefPort` / `routeLabels` verbatim when provided; never
infer per-route values.

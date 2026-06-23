---
description: Generating Gateway API HTTPRoute CRs from Go/Java route-registration code.
applyTo: "**/*.{go,java}"
---

When editing Go (`*.go`) or Java (`*.java`) route-registration code and asked to
generate HTTPRoute CRs from it, convert route registrations to HTTPRoute YAML, or
extract routes from those files, apply the `httproute-from-code` skill.

# public-gateway-authz-policy

An APM package for the public-gateway access-control step of the Core Mesh →
Istio migration. On Istio, Cloud-Core ships a deny-unless-permit
`AuthorizationPolicy` on `public-gateway`, so a service's routes are not
reachable there until the service brings its own `ALLOW` policy. This package's
**default behavior is to recommend writing that policy by hand** — the service
owner is the only one who reliably knows which endpoints are meant to be
public. It generates a draft `AuthorizationPolicy` only when the user
explicitly insists.

## Install

```sh
apm install Netcracker/qubership-core-control-plane/agent-packages/public-gateway-authz-policy --target claude
```

## What you get

- The [`SKILL.md`](.apm/skills/public-gateway-authz-policy/SKILL.md) — the
  manual-authoring recommendation (with a template and authoring tips), the
  candidate-collection procedure (from sibling skills' `accessControl` report
  blocks and from Java `@Route` annotations), and the draft-generation
  procedure used only on request.

## Usage

Invoke the skill by name against a chart, e.g. "run public-gateway-authz-policy
on helm-templates/my-service", or let the
[`core-mesh-to-istio-migration`](../core-mesh-to-istio-migration) orchestrator
call it as a sub-skill after [`core-mesh-crs-to-istio`](../core-mesh-crs-to-istio)
and [`httproute-from-code`](../httproute-from-code) have run, feeding it their
`accessControl` report blocks.

By default (no explicit insistence from the user) this skill **only
recommends** — it prints the manual-authoring template and the collected
candidates, and does not write any file. It writes a draft
`AuthorizationPolicy` to the chart only when told to generate one anyway.
Services with no public endpoints need no policy at all.

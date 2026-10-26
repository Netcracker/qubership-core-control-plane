# regex-routes-migration

A shared APM package holding one procedure: how to migrate legacy Cloud-Core
Mesh routes whose paths contain `{variables}` (legacy regex routes) and
forbidden routes (`allowed: false`, `Forbidden: true`) to Istio without
breaking routing.

Istio has no regex rewrite and ranks regex matches below every `PathPrefix`, so
the procedure follows the chosen option of
[regex-routes-migration.md](../../docs/istio/regex-routes-migration.md):

- every path is cut before its first variable and becomes a `PathPrefix`, with
  the rewrite kept as `ReplacePrefixMatch`;
- forbidden paths on `public-gateway-service` and `private-gateway-service`, and
  the paths the cut newly exposes there, become `AuthorizationPolicy` DENY rules;
- routes whose behavior would change are reported as questions, never silently
  converted. No `VirtualService` is generated.

It is a dependency of [`core-mesh-crs-to-istio`](../core-mesh-crs-to-istio) and
[`httproute-from-code`](../httproute-from-code), so the rules live in exactly one
place.

## Install

You normally don't install this directly — it is pulled in automatically as a
dependency of the two consumer packages. To install it on its own:

```sh
apm install Netcracker/qubership-core-control-plane/agent-packages/regex-routes-migration --target claude
```

## What you get

- The [`SKILL.md`](.apm/skills/regex-routes-migration/SKILL.md) — definitions,
  the row command and the per-gateway worksheet — with three co-located files,
  each short enough to be read whole:
  [`procedure.md`](.apm/skills/regex-routes-migration/procedure.md) (Steps 1–7,
  conflict questions, DENY rules),
  [`render.md`](.apm/skills/regex-routes-migration/render.md) (rendering, the
  `AuthorizationPolicy` template, self-check), and
  [`worked-example.md`](.apm/skills/regex-routes-migration/worked-example.md).

## Model guidance

The per-row steps are mechanical (a POSIX `awk` command computes the cut,
template, legacy length and rewrite), and Haiku-class models apply them reliably.
The pairwise checks (coverage, route conflicts, DENY `notPaths`) compare paths
segment by segment; for charts with nested or shared cuts, or forbidden routes
on the public / private gateways, use a Sonnet-class model or have a human check
the worksheet sections the skill points to in `needsReview`.

## Usage

This skill is not triggered directly by users. Consumer skills fill the
worksheet with their routes and then follow
`../regex-routes-migration/SKILL.md` (a sibling skill once installed) before
they emit HTTPRoutes.

# path-specificity-sorting

A small shared APM package holding one procedure: how to sort Gateway API
`HTTPRoute` `rules[]` by **path specificity** (most specific match first). It is a
dependency of both [`core-mesh-crs-to-gatewayapi`](../core-mesh-crs-to-gatewayapi)
and [`httproute-from-code`](../httproute-from-code) so the rule lives in exactly
one place.

## Install

You normally don't install this directly — it is pulled in automatically as a
dependency of the two consumer packages. To install it on its own:

```sh
apm install Netcracker/qubership-core-control-plane/agent-packages/path-specificity-sorting --target claude
```

## What you get

- The [`SKILL.md`](.apm/skills/path-specificity-sorting/SKILL.md) — the
  specificity ordering rules (segment count, then length, then lexicographic),
  worked example, and why ordering matters across Envoy/Nginx/Istio.

## Usage

This skill is not triggered directly by users. Other skills invoke it as a
sub-procedure by reading `../path-specificity-sorting/SKILL.md` (a sibling skill
once installed) before they emit an HTTPRoute.

# Manual review triggers and Output Summary

Part of the [`core-mesh-crs-to-istio`](SKILL.md) skill.

## Fields that MUST be flagged with `⚠ MANUAL REVIEW`

Each mapping file owns the triggers for the CR it converts, next to the rules they qualify, so a
trigger and its mapping cannot drift apart:

| CR | Triggers |
|---|---|
| `FacadeService` | [facade-service-mapping.md](facade-service-mapping.md) |
| `RouteConfiguration` | [route-configuration-mapping.md](route-configuration-mapping.md) |
| `TlsDef` and egress destinations | [tls-def-mapping.md](tls-def-mapping.md) |
| `StatefulSession` (standalone) | [stateful-session-mapping.md](stateful-session-mapping.md) |
| `StatefulSession` (rule-level) | [stateful-session-rule-mapping.md](stateful-session-rule-mapping.md) |
| `LoadBalance` | [load-balance-mapping.md](load-balance-mapping.md) |
| `HttpFilters` / Lua | [lua-filter-mapping.md](lua-filter-mapping.md) |

One trigger belongs to no single CR:

| Source | Trigger |
|---|---|
| Any template helper | `{{- include ... }}` renders mesh CRs — the helper is out of scope, so its output is unconverted |

## Output Summary (report after completion)

Write the contract report file first (see [Contract → Outputs](contract.md#outputs)), then
print this summary in chat:

```
Transformation complete.

Files modified:     <list> (Core condition wrapper added)
Files generated:    <list> (Istio resources)

Resources transformed:
  FacadeService             → Service (<N> instances)
  Gateway/ingress/egress    → Istio Gateway + HTTPRoute (<N> instances)
  Gateway/mesh              → omitted, east-west HTTPRoute only (<N> instances)
  RouteConfiguration        → HTTPRoute (<N> instances)
  StatefulSession           → DestinationRule (<N> instances)
  LoadBalance               → DestinationRule (<N> instances)
  Lua filters               → TrafficExtension (<N> instances)
  TlsDef                    → Secret + TLS DestinationRule (<N> instances)
  Egress external hosts     → ServiceEntry (<N> instances)
  Forbidden / exposed paths → AuthorizationPolicy DENY (<N> instances)
  Route conflicts           → unresolved (<N>), see report
  Skipped (no cookie / disabled / no policies / no luaFilters / disabled TlsDef): <N>

Detected backend reference (for code-generated HTTPRoutes / Maven plugin):
  backendRefName: <name or "unresolved">
  backendRefPort: <port or "unresolved">
  # if unresolved, state why: no RouteConfiguration destinations found
  #                           | conflicting backends: <list of name:port>
  #                           | all destinations excluded (egress-external or
  #                             platform gateway) — nothing to detect, not a failure

Detected output labels (for Maven plugin / code-generated HTTPRoutes):
  labels: <k1=v1, k2=v2, ... or "unresolved">
  # if unresolved, state why: helper indirection not resolvable
  #                           | conflicting label definitions

Items needing manual review:
  <list every ⚠ MANUAL REVIEW hit — one line per hit>
```

---


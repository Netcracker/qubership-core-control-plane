# Render, self-check and review triggers

Part of the [`regex-routes-migration`](SKILL.md) skill.

### Step 8 — Render

The consumer renders its own HTTPRoutes. A row that is in several worksheets
(one route on several gateways) is rendered **once**, in its owner HTTPRoute,
which attaches to all of those gateways. Pick its rendering by the first match:

1. `review` in any worksheet → nothing but its comment.
2. `keep` → the consumer's normal mapping (`Exact` / `RegularExpression`).
3. `held` or `none` in any worksheet where the row is `allowed: yes` → nothing.
4. `exact` in any worksheet → the two `Exact` rules.
5. `rule` in any worksheet → the rule.
6. Otherwise (`merged→…`, `deny`, implicit `none`) → nothing.

So a forbidden literal row that is `deny` on public and `rule` on internal is
rendered as the rule without `backendRefs` (harmless on public, where the DENY
rule answers first), and a row held on one gateway is held on all of its
owner's gateways — the conflict question names them.

| `emit` | Rendered |
|---|---|
| `rule` (allowed row) | one rule: `PathPrefix <cut>`; when `rewrite` is set, `URLRewrite` `ReplacePrefixMatch <R>`; the consumer's backend, headers and filters |
| `rule` (forbidden row, Step 7) | one rule: `PathPrefix <cut>` and its headers — no `filters`, no `backendRefs` |
| `keep` | the consumer's normal mapping |
| `merged→<#>` | nothing — row `<#>`'s rule serves it |
| `exact` | two `Exact` rules (Step 4) |
| `held` / `none` / `deny` | nothing |
| `review` | no rule |

Every `⚠ MANUAL REVIEW` note of a row becomes a `# ⚠ MANUAL REVIEW` comment
**directly under `rules:`** of the owner HTTPRoute, once per row, in row order —
except a conflict answered `manual`, whose comment goes directly above the rule
it concerns (for a DUP: under `rules:`). Notes about a DENY rule go directly
under the policy's `rules:`. Copy each note into the consumer's `needsReview`.

A `:method=M` header is rendered as `method: M` in the match, never as a
`headers` entry (Gateway API rejects `:method` as a header name).

**AuthorizationPolicy** — one per (`owner`, gateway) that has DENY rules, written
after the owner's HTTPRoute, inside the same Istio guard. Rules sorted by their
first `paths` entry. Name: given by the consumer
(`<HTTPRoute name>-deny-public` / `-deny-private`, or
`<microservice-name>-source-code-deny-public` / `-deny-private`). Labels: the
owner HTTPRoute's labels. `ports`: always `["8080"]` — the platform gateways
listen on 8080 — unless the source states the listener port explicitly (CR
`spec.listenerPort`); never take it from anywhere else. `paths`, `notPaths`,
`methods` and `notMethods`: copied verbatim from the worksheet's `## DENY rules`
section. Use **only** this template — no `from` / `principals`, no
`methods: ["*"]`, no glob `*` in paths, `apiVersion` exactly
`security.istio.io/v1`:

```yaml
---
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: <name>
  labels: <owner HTTPRoute labels — omit when it has none>
spec:
  targetRefs:
  - group: gateway.networking.k8s.io
    kind: Gateway
    name: <public-gateway | private-gateway>
  action: DENY
  rules:
  - to:
    - operation:
        ports: ["8080"]
        paths:
        - "<T(f)>"
        - "<T(f)>/{**}"
        notPaths:          # only when non-empty
        - "<T(A)>"
        - "<T(A)>/{**}"
        methods: ["GET"]   # only when f has :method
  - to:
    - operation:
        ports: ["8080"]
        paths:
        - "<T(A)>"
        - "<T(A)>/{**}"
        notMethods: ["GET"] # only for the extra method rule of 6c.3
```

`targetRefs.name`: `public-gateway-service` → `public-gateway`,
`private-gateway-service` → `private-gateway`.

**Once per run**, when at least one DENY rule was rendered, the consumer adds
these three `needsReview` lines:

- `AuthorizationPolicy path templates ({*}, {**}) require Istio >= 1.22.`
- `meshConfig.pathNormalization.normalization must be MERGE_SLASHES or stronger — otherwise %2F, .. and // bypass the DENY rules.`
- `Denied requests now get 403 "RBAC: access denied" instead of the legacy 404.`

**Once per run**, when any worksheet has a `variable` / `partial` row or a
forbidden row, add this `needsReview` line too (listing the worksheet files):

- `Check the ## Coverage, ## Pair checks and ## DENY rules sections of <worksheets>: route conflicts and DENY notPaths depend on segment-by-segment path comparisons — confirm each verdict before merging.`

### Step 9 — Self-check

Before rendering, re-read every worksheet and confirm:

- [ ] Every row has `kind`, `L`, `cut`, `T` and `R` copied from the row command,
      and an `emit`.
- [ ] `## Row command output` holds the raw output, and every `kind`, `L`, `cut`,
      `T`, `R` in the table equals it — compare line by line.
- [ ] `emit: review` is set on exactly the rows the command marked `review`.
- [ ] Every worksheet has its `## Row command output`, `## Coverage`,
      `## Pair checks`, `## Conflicts` and `## DENY rules` sections; every
      `exposure` line of `## Coverage` has an `X<n>` row (public / private).
- [ ] Every `below` / `covers` verdict in those sections lists the segment counts,
      and no path with fewer segments is marked below a longer one.
- [ ] Every forbidden and exposure row on public / private has a row in
      `## DENY rules`, listing every allowed row below it as a checked candidate
      (rows with `emit` `review` or `held` included).
- [ ] Every explicit forbidden row on another gateway has `covered by <#>` or
      `not covered` in `notes`, plus a `⚠ MANUAL REVIEW` note when covered.
- [ ] Every AuthorizationPolicy has `ports: ["8080"]` (or the CR's
      `spec.listenerPort`) and its rules sorted by their first `paths` entry.
- [ ] No rendered match is `RegularExpression` except `regex` rows, and each of
      those has `# ⚠ MANUAL REVIEW`.
- [ ] Every allowed variable row has `R` or `emit: review`.
- [ ] Every DUP row is `held` (or resolved), and every conflict is in the report
      or was answered in chat.
- [ ] DENY rules exist only for `public-gateway-service` / `private-gateway-service`.
- [ ] Every DENY `paths` pair is `T` and `T/{**}`; every `notPaths` entry comes
      from a candidate that `wins` over the forbidden row.
- [ ] No `VirtualService` was generated.

---

### `⚠ MANUAL REVIEW` triggers

| Trigger | Step |
|---|---|
| raw legacy `regExp` match | 1 |
| variable rewrite that `ReplacePrefixMatch` cannot express | 2 |
| conflict answered `manual` | 5 |
| DENY rule with a partial segment, or a header other than `:method` | 6c |
| `exact` row below a DENY rule | 1 / 6 |
| forbidden route on a gateway without DENY policies that Istio now routes | 7 |

See [`worked-example.md`](worked-example.md) for a complete run.

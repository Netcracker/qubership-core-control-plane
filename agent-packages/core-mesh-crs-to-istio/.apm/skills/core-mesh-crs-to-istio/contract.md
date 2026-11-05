# Contract — core-mesh-crs-to-istio

Part of the [`core-mesh-crs-to-istio`](SKILL.md) skill.

### Inputs

| Input | Type | Required | Notes |
|---|---|---|---|
| `chartPath` | path | yes | Chart or templates folder to transform |
| `interactive` | bool | no | `true` only when a user invokes the skill directly in their own session; orchestrators and sub-agent wrappers pass `false` |
| `resolutions` | map `<unresolved id>: <answer>` | no | Answers to a previous run's `unresolved:` entries |

With `interactive: false` (the default for orchestrated and sub-agent runs),
never ask the user: every blocking question becomes an `unresolved:` entry.
Skip only the work that depends on the answer, continue with everything else,
and set `status: partial` when writing the final report. With
`interactive: true`, ask blocking questions in chat and wait for the answer. A
sub-agent has no user channel — its questions die in its transcript — so a
delegated run must always be `interactive: false`.

### Outputs

In addition to the chat Output Summary, write a machine-readable report to
`.mesh-migration/reports/core-mesh-crs-to-istio.yaml` (create the directory, and ensure
`.mesh-migration/` is listed in the repo's `.gitignore` — reports are working
files, never committed). In an orchestrated run the orchestrator creates the
directory and the `.gitignore` entry, and this skill writes only the report. In a
direct run it does both itself, which is the one edit it makes outside
`chartPath`:

```yaml
reportSchema: 1
skill: core-mesh-crs-to-istio
status: complete            # complete | partial (unresolved items block part of the output)
generatedAt: <ISO-8601>
filesModified: [<paths>]
filesGenerated: [<paths>]
worksheets: [<.mesh-migration/work/core-mesh-crs-to-istio-<gateway>.md, one per gateway>]
resources:
  facadeService: <N>
  gatewayIngressEgress: <N>
  gatewayMesh: <N>
  routeConfiguration: <N>
  statefulSession: <N>
  loadBalance: <N>
  luaFilters: <N>
  tlsDef: <N>
  serviceEntry: <N>
  authorizationPolicy: <N>
  skipped: <N>
backendRef:
  name: <value or null>
  port: <value or null>
  unresolvedReason: <string or null>
labels:
  values: <map or null>
  unresolvedReason: <string or null>
unresolved:                 # empty when status is complete
  - id: gateway/<gateway-name>
    question: "Gateway '<gateway-name>' is referenced in routes but not defined in this chart — ingress or mesh?"
    options: [ingress, mesh]
    default: null
    referencedBy: [<CR names>]
  - id: route-conflict/<#A>-<#B>       # from regex-routes-migration Step 5
    question: "<verbatim question template>"
    options: [keep-<#A>, keep-<#B>, manual]   # or [accept, manual]
    default: <option or null>
needsReview:
  - <one line per ⚠ MANUAL REVIEW hit>
```

Consumers must ignore unknown report fields. A consumer that sees a
`reportSchema` newer than its own documentation must stop and report a contract
mismatch instead of guessing field meanings.

### Side effects

Modifies only:

- mesh-CR files and their `-istio` siblings, under `chartPath`
- `values.yaml` and `values.schema.json`, under `chartPath`
- the report file at `.mesh-migration/reports/core-mesh-crs-to-istio.yaml`
- the route worksheets at `.mesh-migration/work/core-mesh-crs-to-istio-<gateway>.md`
  (see [regex-routes-migration](../regex-routes-migration/SKILL.md))
- `.gitignore`, to add `.mesh-migration/` if it is not already listed — the only
  permitted edit outside `chartPath`, and only that one line. A direct run makes
  it; an orchestrated run leaves `.gitignore` to the orchestrator

Nothing else. This list is the boundary — if a rule elsewhere appears to ask for
a write not on it, the list wins and the rule is wrong.

---


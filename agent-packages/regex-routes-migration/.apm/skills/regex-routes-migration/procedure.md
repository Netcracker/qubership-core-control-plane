# Procedure — Steps 1–7

Part of the [`regex-routes-migration`](SKILL.md) skill. Definitions (`cut`, `T`, `L`, `below`, `covers`, `wins`, `sameBehavior`) and the row command are in [`SKILL.md`](SKILL.md).

Run Steps 1–7 for each gateway worksheet, then Step 8 ([`render.md`](render.md)) once.

### Step 1 — Classify

Run the [row command](SKILL.md#the-row-command) once for the worksheet and copy `kind`,
`L`, `cut`, `T` and `R` of every row into the table.

- `exact` rows (legacy `path` match): `emit: keep` — rendered by the consumer's
  normal `Exact` mapping. Skip them in Steps 2–6. If an `exact` row's path is
  `below` the `paths` of a DENY rule from Step 6, add `⚠ MANUAL REVIEW` to that
  rule's notes (legacy ranked `path` matches last).
- `regex` rows (raw legacy `regExp`): `emit: keep` — rendered by the consumer's
  normal `RegularExpression` mapping — plus the note `⚠ MANUAL REVIEW: <path> is
  a regex match: Istio ranks it below every PathPrefix and cannot rewrite it`.
  Skip them in Steps 2–6.
- `partial` rows are routed like `variable` rows. They only become a problem
  inside a DENY rule (Step 6).

### Step 2 — Rewrite of allowed rows

The row command already decided: set `emit: review` on **exactly** the rows whose
last output column is `review` (their `R` is `✗`) — on no other row — and add
the note:

`⚠ MANUAL REVIEW: <path> needs a regex rewrite (rewrite <rewrite>); not emitted —
its requests now reach the rule that owns <cut>`

Every other row keeps going, even when its rewrite looks unusual. What the
command prints, for orientation only — do not re-derive it:

| `path` | `rewrite` | `R` printed | why |
|---|---|---|---|
| `/api/v1/svc/{id}` | `/svc/{id}` | `/svc` | the rewrite ends with the path's tail after `cut` (`{id}`) |
| `/api/v1/shop/items/{id}/reviews` | `/items/{id}/reviews` | `/items` | tail `{id}/reviews` matches |
| `/api/v1/svc/{a}/x/{b}` | `/{a}/x/{b}` | `/` | tail `{a}/x/{b}` matches, nothing left before it |
| `/api/v1/svc/{id}/items` | `/items/{id}` | `✗` | tail `{id}/items` ≠ `items/{id}` |
| `/api/v1/svc/{id}/x` | `/x` | `✗` | the rewrite drops the variable |

Forbidden rows get `R` = `-`.

### Step 3 — Merge equal rules

Group the allowed rows with `emit` still empty by (`cut`, `headers`). Inside a
group, split the rows by (`behavior`, `R`) — for rows with the same `cut`, equal
(`behavior`, `R`) is exactly `sameBehavior`:

- the first row of each split — or the row the consumer's merge rule names as
  leader: `emit: rule`;
- every other row of that split: `emit: merged→<leader #>`.

Merge only rows the consumer allows to share a rule (same owner, or the
consumer's own merge rule). The merged rule gets the **largest** timeout of its
rows; when timeouts differ, note it — the consumer adds a `needsReview` line.

A group with **more than one** split cannot be one `PathPrefix`: Step 5.1 turns
it into a DUP conflict.

### Step 4 — Header-matched root route → Exact

Legacy example: `/api/v1/svc/resource` with `:method=GET` → `another-service`,
plus `/api/v1/svc/resource/{id}` → `my-service`. In legacy the GET route only
receives the bare root, because `{id}` is longer for everything below it. In
Istio both become `PathPrefix /api/v1/svc/resource` and the method match wins
for every GET below it.

For every allowed `literal` row `Q` with headers: when there is an allowed
`variable` row `V` **without** headers such that `V.path` = `Q.path` + `/{name}`
(exactly one more segment, a variable) and **not** `sameBehavior(Q, V)`:

- set `Q.emit: exact`;
- the consumer emits `Q` as **two** `Exact` rules with `Q`'s headers and
  behavior: one on `Q.path` and one on `Q.path` + `/`. When `Q.rewrite` is set,
  each uses `ReplaceFullPath`: `Q.rewrite` and `Q.rewrite` + `/`
  (`ReplacePrefixMatch` is not allowed with `Exact`). Without a rewrite, no
  `URLRewrite` filter.

Rows with `emit: exact` never conflict and are skipped in Step 5.

### Step 5 — Conflicts

Only allowed rows take part, and never rows with `emit` `review`, `exact` or
`keep`. Two
literal rows never conflict — they behave the same in both meshes — so every
check below involves at least one `variable` / `partial` row. A pair whose
headers are not compatible never conflicts. "Strictly below" for cuts means the
segments of the shorter `cut` are the first segments of the longer one and the
cuts are not equal.

Run the four checks in order. A pair of rows gets at most **one** conflict —
skip a pair that an earlier check already recorded.

**5.1 DUP** — every Step 3 group with more than one split, when at least one row
of the group is `variable` / `partial`. One conflict per group; its rows are the
first row of each split.

**5.2 HEADERS** — every pair `H`, `V` with `H.cut` = `V.cut`, different headers,
at least one of them `variable` / `partial`, and not `sameBehavior(H, V)`. `H`
is the row with more header matchers.

**5.3 Coverage and EXPAND** — fill `## Coverage`, one line per allowed
`variable` / `partial` row `V` (any `emit` except `review`):

1. Write `cut(V)` and its segment count `n`.
2. Candidates: every allowed row `C` **without headers**, `C ≠ V`,
   `C.path ≠ /`, any `emit` — but only rows with **at most `n` segments** (a
   longer path never covers `cut(V)`). For each, write the position check of
   `covers(C, cut(V))`.
3. `W` = the covering row with the largest `L`.
4. Result:
   - no covering row → `exposure` (Step 6b uses it);
   - `W.cut` = `cut(V)` → `none` (a merge or a DUP);
   - `sameBehavior(W, V)` → `none`;
   - otherwise → **EXPAND (`W`, `V`)**: the cut now takes requests `W` served in
     legacy.

| V | cut(V) (segments) | covering rows checked | winner W | result |
|---|---|---|---|---|
| `/api/v1/store/{id}/items` | `/api/v1/store` (3) | `/api/v1/store` (3): all equal → covers; `/api/v1/store/public` (4) → too long | `/api/v1/store` | `none` (same cut) |
| `/api/v1/store/orders/{id}/pdf` → `/pdf/orders/{id}/pdf` | `/api/v1/store/orders` (4) | `/api/v1/store` (3): covers | `/api/v1/store` → `/store` | EXPAND: `join(/store, orders)` = `/store/orders` ≠ `/pdf/orders` |
| `/api/v1/report/{id}/csv` | `/api/v1/report` (3) | `/api/v1/report/public` (4) → too long | — | `exposure` |

**5.4 STEAL** — fill `## Pair checks`, one line per pair: `A` is an allowed
`variable` / `partial` row (any `emit` except `review`), `B` is any other allowed
row (not `review` / `exact` / `keep`) whose `cut` is **longer** than `A.cut` and
**starts with** `A.cut`'s segments. Result **STEAL (`A`, `B`)** when
`overlaps(A.path, B.path)`, `wins(A, B)`, and not `sameBehavior(A, B)`: `B`'s
longer prefix now takes requests `A` won in legacy. Otherwise `none`.

| A | B | B.cut longer, starts with A.cut? | overlaps(A.path, B.path)? | L(A) vs L(B) | same behavior? | result |
|---|---|---|---|---|---|---|
| `/api/v1/store/{tenant}/export` (cut `/api/v1/store`) | `/api/v1/store/public` | yes | `{tenant}`–`public` variable → yes | 22 > 20 | no | STEAL |
| `/api/v1/store/{id}` (cut `/api/v1/store`) | `/api/v1/store/orders/{x}/items` | yes | `{id}`–`orders` variable → yes | 15 < 28 | — | `none` (A does not win) |

**Sample request** (used in the questions):

- DUP / HEADERS: `B.path` with every variable replaced by `x`, plus `B`'s headers.
- EXPAND: `cut(V)` when `V.path` has exactly one segment after `cut(V)`;
  otherwise `cut(V)` + `/x`.
- STEAL: `A.path` where every variable segment takes the segment of `B.path` at
  the same position when `B.path` has a literal there, otherwise `x`
  (`A = /api/v1/catalog/{tenant}/export`, `B = /api/v1/catalog/public` →
  `/api/v1/catalog/public/export`).

**What to do with each conflict:**

| Case | Emission until resolved | Options | Default |
|---|---|---|---|
| DUP | **hold** every row of the group (`emit: held`) — nothing is emitted for this `cut` | `keep-<#>` for the leader of each split, `manual` | `keep-<leader #>` of the split that contains the `literal` row whose path equals `cut`, else `null` |
| HEADERS / EXPAND / STEAL | emit as decided above | `accept`, `manual` | `null` |

Conflict id: `route-conflict/<#A>-<#B>` — the row ids in ascending order (more
ids with `-` for a DUP with three behaviors). The same pair on several gateways is
**one** conflict; list all its gateways in the question.

- **`interactive: true`** → ask the question in chat and wait.
- **`interactive: false`** → add an `unresolved:` entry `{id, question, options,
  default}` to the consumer's report and finish with `status: partial`.

Applying the answer (in this run, or on a follow-up run that receives it through
`resolutions`):

| Answer | Effect |
|---|---|
| `keep-<#>` | emit the rule of row `#` (and its merged rows); the other rows of the DUP get `emit: none` and a `needsReview` line naming the requests that changed behavior |
| `accept` | nothing changes; add a `needsReview` line recording the accepted behavior change |
| `manual` | DUP: every row of the group gets `emit: none`, and the comment goes directly under `rules:`. Other cases: the rules stay, and the comment goes directly above the rule of the conflict's **variable** row (`V` for EXPAND, `A` for STEAL, the variable row for HEADERS — the lower id when both are variable; the rule it is merged into when it is `merged→…`). Comment and `needsReview` line: `⚠ MANUAL REVIEW: <conflict id> — <one-line summary of the changed requests>; handle by hand — e.g. VirtualService on the waypoint (docs Option 3)` |

Question templates — fill every `<…>`, keep the rest verbatim:

- **DUP** — `Routes <#A> <A.path> (<A behavior>) and <#B> <B.path> (<B behavior>) on <gateways> both become PathPrefix <cut> in Istio: Istio has no regex routes, so every path is cut before its first {variable}, and one PathPrefix can have only one behavior. Which behavior should <cut> keep? keep-<#A> | keep-<#B> | manual (emit neither and handle it by hand).`
- **HEADERS** — `On <gateways>, route <#H> <H.path> with <H.headers> and route <#V> <V.path> both become PathPrefix <cut>. Istio prefers the rule with the header match for every request below <cut>, so <sample> moves from <#V> (<V behavior>) to <#H> (<H behavior>). accept (keep both rules and this change) | manual.`
- **EXPAND** — `On <gateways>, route <#V> <V.path> becomes PathPrefix <cut(V)>, so Istio sends every request below <cut(V)> to it. In legacy route <#W> <W.path> served requests like <sample>: legacy → <W behavior>, Istio → <V behavior>. accept (keep the rule and this change) | manual.`
- **STEAL** — `On <gateways>, route <#B> <B.path> is a longer PathPrefix than <A.cut> (from route <#A> <A.path>), so Istio sends requests like <sample> to <#B> (<B behavior>), while legacy sent them to <#A> (<A behavior>). accept (keep both rules and this change) | manual.`

Write a behavior as `backend <name:port>, rewrite <cut> → <R>` (add
`host rewrite`, header changes when they differ).

### Step 6 — DENY rules (public and private gateways only)

Run this step only for `public-gateway-service` and `private-gateway-service`.

**6a — Forbidden rows.** Set `emit: deny` on:

- every `explicit` forbidden row;
- every `implicit` forbidden row that is **covered**: some allowed row with `emit`
  `rule`, `merged→…` or `held` (kind `literal` / `variable` / `partial`) has
  `below(row.path, thatRow.cut)` — Istio would route it. Implicit forbidden rows
  that are not covered get `emit: none` (nothing routes them).

**6b — Exposure rows.** Every `## Coverage` line with result `exposure` (its
`V` has `emit` `rule`, `merged→…` or `held`) means the cut exposes paths legacy
never routed. Add a synthetic row `X<n>` to the table with `path` = `cut(V)`,
`forbidden: exposure`, `owner` = `V.owner`, `emit: deny`, and `L` from the row
command. Add each `cut` once.

**6c — One DENY rule per forbidden row `f`** (rows with `emit: deny`; rows with
the same `T` and headers produce one rule). Write each rule as a row of the
worksheet's `## DENY rules` section, **listing every candidate you checked** with
its `below` and `wins` result, so a missing `notPaths` entry is visible:

1. `paths`: `T(f.path)` and `T(f.path)` + `/{**}`.
2. Candidates: check **every** allowed row `A` of this worksheet, **whatever its
   `emit`** (`rule`, `merged→…`, `held`, `review` — legacy routed them all),
   except kind `exact` / `regex`. For each, write `segments A / f`, the position
   check and `L A / f` ([below check](SKILL.md#the-below-check)): `A` is **in** when
   `below(A.path, f.path)` and `wins(A, f)`, else **out**. Example for
   `f` = `/api/v1/store/{id}/admin` (5 segments, `L` 21):
   `/api/v1/store/{id}` 4/5 → out; `/api/v1/store/{id}/admin/stats` 6/5, all
   equal, 27/21 → in; `/api/v1/store/orders/{x}/items` 6/5, variables match,
   28/21 → in. When `f` has `:method=M`, keep only
   candidates without a method or with method `M`.
3. For each distinct `T(A)` among the candidates:
   - some candidate with that `T` has **no** headers → add `T(A)` and `T(A)` +
     `/{**}` to `notPaths`;
   - all candidates with that `T` have **only** `:method` headers → add `T(A)` and
     `T(A)` + `/{**}` to `notPaths`, **and**, when `f` has no headers, add an extra
     rule: `paths` `T(A)` and `T(A)` + `/{**}`; `notMethods` = all their methods;
     `notPaths` = the entries of this `notPaths` list that lie strictly below
     `T(A)`;
   - any candidate with that `T` has another header → `⚠ MANUAL REVIEW`, do not
     emit this rule.
4. `f` has `:method=M` → add `methods: [M]`. `f` has any other header →
   `⚠ MANUAL REVIEW`, do not emit this rule.
5. Any `partial` segment in `paths` or `notPaths` → `⚠ MANUAL REVIEW`, do not
   emit this rule (an AuthorizationPolicy path template takes whole segments only).
6. Sort `notPaths` in plain byte order — `{` sorts **after** letters and digits, so
   `/a/public` comes before `/a/{*}/x` — remove duplicates; omit `notPaths` when
   empty.

Why `notPaths`: the policy is checked **before** routing, so without them a DENY
on `/a/{*}/internal` would also block a longer allowed route
`/a/{*}/internal/status` that won in legacy.

### Step 7 — Other gateways

On every gateway that is not `public-gateway-service` / `private-gateway-service`
(internal, egress, mesh, facade, custom ingress) no DENY rules are generated.
Decide each forbidden row with this table and write the result in `emit` and
`notes`:

| Forbidden row | `emit` | `notes` |
|---|---|---|
| `explicit`, kind `literal` | **`rule`** — a rule without `backendRefs` on `PathPrefix <cut>`: Istio answers 404, and the longest `PathPrefix` still wins as in legacy | see check A |
| `explicit`, kind `variable` / `partial` | `none` | see check B |
| `implicit` | `none` | — |

- **Check A** — for each allowed variable row `V` with `overlaps(V.path,
  row.path)`, `row.cut` longer than `V.cut` and starting with it, and
  `wins(V, row)`: add `⚠ MANUAL REVIEW: <row.path> now also returns 404 for
  <sample> that legacy routed to <#V>` (sample as for STEAL).
- **Check B** — is the row covered? (some allowed row with `emit` `rule`,
  `merged→…` or `held` has `below(row.path, thatRow.cut)`). Write `covered by <#>`
  or `not covered`. Covered → add `⚠ MANUAL REVIEW: <path> is forbidden in
  legacy, but on <gateway> it is now routed by PathPrefix <covering cut>; no DENY
  policy is generated for this gateway`.

No exposure rows (6b) on these gateways.

Continue with [`render.md`](render.md).

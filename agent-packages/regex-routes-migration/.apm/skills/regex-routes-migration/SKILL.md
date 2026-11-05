---
name: regex-routes-migration
description: >
  Shared procedure for migrating legacy Cloud-Core Mesh routes whose paths contain
  {variables} (legacy regex routes) and forbidden routes (allowed false / Forbidden
  true) to Istio: cut each path to a PathPrefix, keep the rewrite, detect behavior
  conflicts, and turn forbidden paths on public/private gateways into
  AuthorizationPolicy DENY rules. Not triggered directly by users — invoked as a
  sub-procedure by core-mesh-crs-to-istio and httproute-from-code before they emit
  HTTPRoutes.
---

# Shared procedure — path-variable and forbidden routes

Consumer skills (`core-mesh-crs-to-istio`, `httproute-from-code`) fill a
worksheet with their routes and then run this procedure. It decides, for every
route, which HTTPRoute rule to emit, which `AuthorizationPolicy` DENY rules to
emit, and which questions to ask.

## Why

Legacy Cloud-Core Mesh turns a path with variables
(`/api/v1/svc/resource/{id}/items`) into an Envoy regex route, rewrites it with a
regex, and always picks the **longest** matching route. Istio cannot do that:

- Gateway API has no regex rewrite — only `ReplacePrefixMatch` (needs a
  `PathPrefix` match) and `ReplaceFullPath`.
- Istio always ranks `Exact` above `PathPrefix` above `RegularExpression`, whatever
  their length, and this cannot be configured.

So every route becomes a `PathPrefix` cut before its first variable. The cut
route also matches paths it did not match in legacy, and forbidden routes cannot
be expressed by routing alone. This procedure repairs both.

## Files of this skill — read all four

This skill is split into four files so that each one can be read whole. **Read
every file in full with the Read tool, in this order, before you fill the
worksheet** — do not work from memory:

| File | Content |
|---|---|
| `SKILL.md` (this file) | hard rules, definitions, the below check, the row command, the worksheet |
| [`procedure.md`](procedure.md) | Steps 1–7: classify, rewrite, merge, Exact fix, conflicts, DENY rules, other gateways |
| [`render.md`](render.md) | Step 8: how to render rules and the AuthorizationPolicy template; Step 9: self-check; `⚠ MANUAL REVIEW` triggers |
| [`worked-example.md`](worked-example.md) | a complete worksheet and its YAML output |

## Model guidance

The per-row work — the row command, merging, the Exact fix, rendering — is
mechanical, and small (Haiku-class) models do it reliably. The **pairwise
checks** — `## Coverage`, `## Pair checks` and the DENY candidates — compare
paths segment by segment, and small models get them wrong in practice (they
mark a shorter path as below a longer one, or skip a candidate). When a
worksheet has a `variable` row whose `cut` equals, contains or is contained in
another row's `cut`, or any forbidden / exposure row on public / private, run
this procedure with a Sonnet-class model, or have a human check those three
worksheet sections before merging. Step 8 adds a `needsReview` line pointing
at them.

## Hard rules

- **Never generate a `VirtualService`.** When a route cannot be migrated
  without changing behavior, ask (see [Step 5](procedure.md#step-5--conflicts)) or flag
  `# ⚠ MANUAL REVIEW`.
- **Write every computed value into the worksheet. Never compute in your head.**
  Re-read the worksheet before rendering YAML.
- Compute `kind`, `L`, `cut`, `T` and `R` **only** with the
  [row command](#the-row-command) and copy its output into the worksheet. Do not
  correct its values by hand.
- Work **one gateway at a time**. A route that is on several gateways has one row
  in each gateway's worksheet, with the same row id.
- DENY rules are generated **only** for `public-gateway-service` and
  `private-gateway-service`. Every other gateway follows
  [Step 7](procedure.md#step-7--other-gateways).

---

## Definitions

A Helm expression `{{ ... }}` inside a path is **literal text**, never a
variable. Only a single `{name}` is a variable.

| Term | Meaning | Example |
|---|---|---|
| `segments(p)` | split `p` on `/`, drop empty parts | `/api/v1/svc/{id}/` → `api`, `v1`, `svc`, `{id}` |
| variable segment | a segment that is exactly `{name}` | `{id}` |
| partial segment | a segment that contains `{name}` but also other text | `v{version}`, `{name}.txt` |
| `kind` | `literal` (no variables), `variable` (only whole variable segments), `partial` (has a partial segment), `exact` (legacy `path` match), `regex` (legacy `regExp` match) | |
| `cut(p)` | the segments **before** the first segment that contains `{name}`, joined as `/s1/s2/...`; `/` when nothing remains. For a literal path, `cut(p)` = `p` without trailing `/` | `/api/v1/svc/{id}/items` → `/api/v1/svc`; `/{tenant}/x` → `/` |
| `T(p)` (template) | `p` with every variable segment replaced by `{*}` and the trailing `/` removed | `/api/v1/svc/{id}/items` → `/api/v1/svc/{*}/items` |
| `L(p)` (legacy length) | length of `p` with every Helm expression replaced by `H`, every `{name}` replaced by `A`, trailing `/` removed | `/api/v1/svc/{id}/items` → `/api/v1/svc/A/items` → 19 |
| `overlaps(a, b)` | compare `segments(a)` and `segments(b)` position by position, **up to the shorter one**; true when every pair is equal or at least one of the pair is a variable segment | `overlaps(/a/{x}/c, /a/b)` = true; `overlaps(/a/b, /a/c)` = false |
| `below(a, f)` | the request paths of `a` lie at or under `f` — check it with the [below check](#the-below-check) | `below(/a/{x}/c, /a/b)` = true; `below(/a, /a/b)` = false |
| `covers(C, p)` | legacy route `C` matches the request path `p` — it is `below(p, C.path)`: `C` has **no more** segments than `p` | `covers(/a/{x}, /a/b)` = true; `covers(/a/b/c, /a/b)` = false |
| `join(r, s)` | `r` and `s` joined with exactly one `/`; `join(r, "")` = `r` | `join(/x, items)` = `/x/items`; `join(/, items)` = `/items` |
| headers | the route's header matchers, written `name=value` (exact value) or `name~regex` (regular expression) and sorted; `-` when none. `:method=GET` is a method match | `:method=GET` |
| headers compatible | two header lists are compatible unless both set the same header name to different exact values | `:method=GET` vs `:method=POST` → not compatible |

### The below check

`below(a, f)` is true only when **both** steps pass. Write the segment counts
down — the count decides most cases:

1. **Count.** `a` must have **at least as many** segments as `f`. Fewer → false.
   A shorter path is never below a longer one.
2. **Positions.** For each position 1 … (segments of `f`): the segment of `a` and
   the segment of `f` are equal, or one of them is a variable segment. Any other
   pair → false.

| `a` | `f` | segments `a` / `f` | positions | `below(a, f)` |
|---|---|---|---|---|
| `/api/v1/svc/{id}` | `/api/v1/svc/{id}/admin` | 4 / 5 | — | **false** (fewer segments) |
| `/api/v1/svc/{id}/admin/stats` | `/api/v1/svc/{id}/admin` | 6 / 5 | all equal | true |
| `/api/v1/svc/orders/{id}/items` | `/api/v1/svc/{id}/admin` | 6 / 5 | `orders`–`{id}` variable, `{id}`–`admin` variable | true |
| `/api/v1/svc/{id}` | `/api/v1/svc` | 4 / 3 | all equal | true |
| `/api/v1/svc` | `/api/v1/svc` | 3 / 3 | all equal | true |
| `/api/v1/svc/debug` | `/api/v1/svc/{id}/admin` | 4 / 5 | — | **false** (fewer segments) |
| `/api/v1/catalog/public` | `/api/v1/catalog/{x}/export` | 4 / 5 | — | **false** |
| `/api/v1/shop/items/{id}` | `/api/v1/shop/orders` | 5 / 4 | `items`–`orders` differ | **false** |

`covers(C, p)` is the same check with the roles swapped — `below(p, C.path)`:
the request path `p` needs at least as many segments as the route `C`. A route
with **more** segments than `p` never covers it (`/api/v1/catalog/public` does
not cover `/api/v1/catalog`).

`L` reproduces legacy route ordering (control-plane `util/routes/sort.go`): of two
legacy routes that match a request, the one with the larger `L` wins; on equal
`L`, the one with more header matchers wins.

### The row command

Compute `kind`, `L`, `cut`, `T` and `R` for all rows of a worksheet in **one**
batch with this command — never by hand. Put one line per row between the `ROWS`
markers: `#`, `path`, `rewrite` (`-` when none), `allowed` (`yes` / `no`),
separated by **TAB** characters. Leave out rows whose `kind` is `exact` or
`regex` (the consumer sets those from the source).

```bash
awk -F'\t' '
function norm(s) { gsub(/[{][^}]*[}]/, "{*}", s); return s }
{
  id = $1; path = $2; rw = $3; allowed = $4
  p = path; gsub(/[{][{]/, "\001", p); gsub(/[}][}]/, "\002", p)
  n = split(p, raw, "/"); k = 0
  for (i = 1; i <= n; i++) if (raw[i] != "") seg[++k] = raw[i]
  first = 0; kind = "literal"; t = ""; cut = ""
  for (i = 1; i <= k; i++) {
    isvar = (seg[i] ~ /^[{][^}]*[}]$/); hasv = (index(seg[i], "{") > 0)
    if (hasv && !first) first = i
    if (hasv && !isvar) kind = "partial"; else if (isvar && kind == "literal") kind = "variable"
    t = t "/" (isvar ? "{*}" : seg[i])
    if (!first) cut = cut "/" seg[i]
  }
  if (cut == "") cut = "/"; if (t == "") t = "/"
  l = p; gsub(/\001[^\002]*\002/, "H", l); gsub(/[{][^}]*[}]/, "A", l); sub(/\/+$/, "", l)
  if (allowed != "yes") r = "-"
  else if (rw == "-" || rw == "") r = cut
  else {
    q = rw; gsub(/[{][{]/, "\001", q); gsub(/[}][}]/, "\002", q)
    m = split(q, rr, "/"); j = 0
    for (i = 1; i <= m; i++) if (rr[i] != "") rs[++j] = rr[i]
    if (kind == "literal") { r = ""; for (i = 1; i <= j; i++) r = r "/" rs[i]; if (r == "") r = "/" }
    else {
      s = k - first + 1; ok = (j >= s)
      for (i = 1; ok && i <= s; i++) if (norm(rs[j - s + i]) != norm(seg[first + i - 1])) ok = 0
      if (ok) { r = ""; for (i = 1; i <= j - s; i++) r = r "/" rs[i]; if (r == "") r = "/" } else r = "✗"
    }
  }
  out = id "\t" kind "\t" length(l) "\t" cut "\t" t "\t" r "\t" (r == "✗" ? "review" : "-")
  gsub(/\001/, "{{", out); gsub(/\002/, "}}", out); print out
  delete seg; delete rs
}' <<'ROWS'
R1	/api/v1/my-service/resource	/resource	yes
R2	/api/v1/my-service/resource/{var1}/internal-api	-	no
R3	/api/v1/my-service/resource/{var1}/migrated-api	/resource	yes
ROWS
```

Output: one line per row — `#`, `kind`, `L`, `cut`, `T`, `R`, and `review` when
the row must get `emit: review` (else `-`), TAB-separated:

```text
R1	literal	27	/api/v1/my-service/resource	/api/v1/my-service/resource	/resource	-
R2	variable	42	/api/v1/my-service/resource	/api/v1/my-service/resource/{*}/internal-api	-	-
R3	variable	42	/api/v1/my-service/resource	/api/v1/my-service/resource/{*}/migrated-api	✗	review
```

Paste the raw output into the worksheet under `## Row command output`, then copy
each value into the table **unchanged**. The command is the authority: a rewrite
such as `/items/{id}/reviews` for `/api/v1/shop/items/{id}/reviews` **is**
expressible (`R` = `/items`) because its tail matches the path's tail — never
replace a printed `R` with `✗`, or a `✗` with a value.

`wins(A, B)` — legacy prefers `A` over `B`: `L(A.path) > L(B.path)`, or equal `L`
and `A` has more header matchers than `B`.

---

## Input — the worksheet

The consumer skill writes one Markdown worksheet per gateway to
`.mesh-migration/work/<consumer-skill>-<gateway>.md` (create the folder; it is
inside the gitignored `.mesh-migration/`). Columns, in this order:

| Column | Filled by | Content |
|---|---|---|
| `#` | consumer | row id, unique in the run (`R1`, `R2`, … in discovery order). The same source route keeps the same id in every gateway's worksheet |
| `source` | consumer | where the route comes from (`file` + CR name / `file:line`) |
| `owner` | consumer | the HTTPRoute that will hold this row's rule |
| `path` | consumer | the legacy path exactly as written (`prefix` / `from`) |
| `kind` | Step 1 (row command) | `literal` / `variable` / `partial`; the consumer writes `exact` / `regex` itself |
| `headers` | consumer | header matchers, see [Definitions](#definitions) |
| `allowed` | consumer | `yes` — routed on this gateway; `no` — forbidden on this gateway |
| `forbidden` | consumer | `-` when allowed; `explicit` — declared forbidden (CR `allowed: false`, Go `Forbidden: true`, Java `allowed(false)`); `implicit` — code route whose type is narrower than this gateway; `exposure` — synthetic row added by Step 6b |
| `behavior` | consumer | behavior id: rows with identical backend (`name:port`), host rewrite and request header add/remove get the same id (`B1`, `B2`, … in order of first appearance); `-` for forbidden rows |
| `rewrite` | consumer | the legacy `prefixRewrite` / `to`; `-` when none |
| `L` | Step 1 (row command) | legacy length |
| `cut` | Step 1 (row command) | `cut(path)` |
| `T` | Step 1 (row command) | `T(path)` |
| `R` | Step 1 (row command) | the prefix the rule rewrites `cut` to; `cut` itself when there is no rewrite; `✗` when not expressible; `-` for forbidden rows |
| `emit` | Steps 1–7 | the decision: `rule`, `merged→<#>`, `exact`, `keep`, `held`, `deny`, `none`, `review` |
| `notes` | any step | conflict ids, `⚠ MANUAL REVIEW` reasons |

Below the table, every worksheet has five more sections, in this order — the raw
row command output (Step 1) and the sections filled by Steps 5 and 6. Each check
is written down with its intermediate values; rendering copies from them:

```markdown
## Row command output
<the command's output, unchanged>

## Coverage
| V | cut(V) (segments) | covering rows checked | winner W | result |

## Pair checks
| A | B | B.cut longer, starts with A.cut? | overlaps(A.path, B.path)? | L(A) vs L(B) | same behavior? | result |

## Conflicts
| id | case | rows | gateways | sample request | emission until answered | options | default |

## DENY rules
| rule | for row | paths | candidates checked (row: segments cand/f, positions, L cand/f → in / out) | notPaths | methods / notMethods |
```

Write `none` under a section heading when it has no rows.

`sameBehavior(A, B)`, for two allowed rows where `cut(B)` equals `cut(A)` or
lies below it (the segments of `cut(A)` are the first segments of `cut(B)`):
the `behavior` ids are equal **and** `join(A.R, tail) = B.R`, where `tail` is the
segments of `cut(B)` after the segments of `cut(A)` (empty when the cuts are
equal). Example: `A.cut = /api/v1/svc`, `A.R = /svc`, `B.cut = /api/v1/svc/orders`,
`B.R = /svc/orders` → `tail = orders`, `join(/svc, orders) = /svc/orders` → same.
Timeouts are not part of the behavior.

---

---

Continue with [`procedure.md`](procedure.md).

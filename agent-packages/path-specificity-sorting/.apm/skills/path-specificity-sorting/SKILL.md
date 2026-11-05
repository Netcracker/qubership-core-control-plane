---
name: path-specificity-sorting
description: >
  Shared procedure for sorting Gateway API HTTPRoute `rules[]` by path
  specificity (most specific first). Not triggered directly by users — invoked as
  a sub-procedure by other skills (`httproute-from-code`,
  `core-mesh-crs-to-istio`) before they emit an HTTPRoute. Apply when ordering
  HTTPRoute rules so longest/most-specific prefix matches appear first.
---

# Shared rule — sort HTTPRoute rules by path specificity

This is a shared procedure reused by multiple skills and generators
(`httproute-from-code`, `core-mesh-crs-to-istio`, `httproutes-generator-maven-plugin`).
Apply it before emitting
an HTTPRoute so the most specific path match appears first in `rules[]`.

The path to sort on is the path value the rule **emits** — after
[`regex-routes-migration`](../regex-routes-migration/SKILL.md) has cut paths with
`{variables}` to their `PathPrefix`. Never sort on the legacy `from` /
`match.prefix` when it contains a variable: `/api/v1/svc/{id}/items` is emitted,
and sorted, as `/api/v1/svc`. A raw legacy `match.regExp` (flagged
`⚠ MANUAL REVIEW`) is sorted on its regex text.

## Specificity ordering rules

1. Count path segments (split by `/`, ignore empty):
   `/api/v1/users/profile` → 4 segments  
   `/api/v1/users` → 3 segments  
   `/api` → 1 segment

2. More segments = higher specificity = appears first.

3. Tie-break by path length (longer string first).

4. Tie-break by lexicographic order (ascending) for stable output.

5. Equal path value: `Exact` before `PathPrefix`, and a rule with a `method` or
   header match before one without (Gateway API precedence).

## Example — input order does not matter, output is always sorted

Input paths (any order):
```
/api/v1/mesh-test-service-go/1234   → 4 segments
/api/v1/mesh-test-service-go        → 3 segments
/api/v1                             → 2 segments
/api                                → 1 segment
```

Sorted output in `rules[]`:
```yaml
rules:
  - matches:
      - path:
          type: PathPrefix
          value: /api/v1/mesh-test-service-go/1234   # 4 segments — first
  - matches:
      - path:
          type: PathPrefix
          value: /api/v1/mesh-test-service-go         # 3 segments
  - matches:
      - path:
          type: PathPrefix
          value: /api/v1                              # 2 segments
  - matches:
      - path:
          type: PathPrefix
          value: /api                                 # 1 segment — last
```

## Why this matters

GatewayAPI spec defines that more specific prefix matches win regardless of
order, but not all gateway implementations (Envoy, Nginx, Istio) respect this
consistently. Sorting longest-prefix-first ensures correct behaviour across
all implementations.

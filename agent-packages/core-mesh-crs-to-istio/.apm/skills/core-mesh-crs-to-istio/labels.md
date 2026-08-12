## Common labels resolution rules

Rules for labels this skill applies to generated Istio resources and reports in
Step 6 (`labels.values`).

### 1. Copy from the source mesh CR

When the source CR has `metadata.labels`:

1. Copy **all** labels onto every generated Istio resource from that CR
   (`HTTPRoute`, `Gateway`, `DestinationRule`, `TrafficExtension`, `Service`).
2. Replace any label **value** that equals `core-operator` with `istiod`
   (typical key: `app.kubernetes.io/managed-by`).
3. Preserve Helm expressions verbatim (`{{ .Values.* }}`).

If several source CRs contribute different label maps to the same generated
resource, mark labels **unresolved** (do not invent a merge).

### 2. No source labels

If the source CR has no `metadata.labels`, omit `metadata.labels` on the
generated resource (do not invent a default set).

### 3. Reporting (Step 6)

- Resolved map → report under `labels.values`.
- Helper indirection, conflicting maps, or no labels to report →
  `labels.values: null` and set `labels.unresolvedReason`. Do not invent
  missing keys.

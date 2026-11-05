# Migration log and final report

Part of the [`core-mesh-to-istio-migration`](SKILL.md) skill.

## Migration log — MANDATORY

The skill **must** create and continuously update a migration log next to the
sub-skill reports:

```
.mesh-migration/MIGRATION_LOG.md
```

The log is the single source of truth for what the automation did. It is updated
**after every step** — never wait until the end. If the log file cannot be
written for any reason, stop immediately and report the failure to the user.
Like the reports, the log is a working file inside the gitignored
`.mesh-migration/` folder; a full run starts a fresh log, a resumed run appends
to the existing one.

### Log structure

````markdown
# Core Mesh → Istio Migration Log

Started: <ISO-8601 timestamp>
Chart:   <chart path>
Code:    <code path>
Language: <Go | Java | Go+Java>

---

## Done
**Items fully applied by automation. One bullet per concrete change.**

## Skipped
**Items intentionally not applied, with reason.**

## Needs review
**Items the user MUST verify before merging. Each entry MUST include:**
**- File / location**
**- Why it needs human review**
**- Suggested action**

## Per-step status

| Step | Title                                       | Status      | Notes |
|------|---------------------------------------------|-------------|-------|
| 1    | Migrate mesh CRs → HTTPRoute CRs            | pending     |       |
| 1.1  | Log manually handle flagged features        | pending     |       |
| 2.1  | Switch to mesh-aware route libraries        | pending     |       |
| 2.2  | Set SERVICE_MESH_TYPE env var               | pending     |       |
| 2.3  | Add Maven plugin (Java only)                | pending     |       |
| 2.4  | Generate HTTPRoute CRs from code            | pending     |       |
| 2.5  | Verify HTTPRoutes are Istio-guarded         | pending     |       |
| 2.6  | Detect duplicate HTTPRoute rules            | pending     |       |
| 2.7  | Flag imperative control-plane calls         | pending     |       |

## Commands run

| Step | Command | Exit code | Notes |
|------|---------|-----------|-------|
````

> **Note:** The log uses bold text (not HTML comments) for section descriptions
> so they are preserved across all Markdown renderers and are re-parseable by
> the agent on idempotent reruns.

### Logging rules

- **Do:** append concrete file paths, resource names, counts, and commands you ran.
- **Do:** classify every non-trivial action as **Done**, **Skipped**, or **Needs review**.
- **Do:** record every command and its exit code in the **Commands run** table.
- **Do:** echo a short chat summary of the log update after each step
  (`Updated MIGRATION_LOG.md — 3 done, 1 needs review`).
- **Don't:** overwrite the log — always append.
- **Don't:** delete a `Needs review` entry until the user confirms it is resolved.

### What belongs in each bucket

**Structural blockers / flagged conversions** — Copy every `needsReview:` line and every `# ⚠ MANUAL REVIEW` hit from
sub-skill reports into **Needs review**.

**Unknown values** — values the agent cannot safely infer and must not guess
(orchestrator / wiring concerns, not CR field mapping):

| Item | Example location |
|------|-----------------|
| Unresolved gateway references (`unresolved:` from Step 1) | HTTPRoute `parentRefs` |
| Missing microservice name (placeholder `<microservice-name>` in output) | Generated HTTPRoute / `source-code-httproutes.yaml` |
| Ambiguous Java route-registration artifact (webclient vs resttemplate) | `pom.xml` |
| Unknown library versions | `pom.xml` / `go.mod` |

**Done** examples: files wrapped in Core/Istio guards, generated `-istio.yaml`
files, HTTPRoutes emitted from code, Maven plugin added, env var wired, library
versions bumped, `values.yaml` / `values.schema.json` updated, commands that
exited 0.

**Skipped** examples: Maven plugin for a Go-only service, library swap for a
language not present, a step the user explicitly said to defer, optional build
commands not available in the environment.

---

## Final checklist and hand-off

Before declaring the migration complete, produce a **Final report** that mirrors
the "Final Checklist" in the migration guide. Mark `[x]` only when the step has
at least one **Done** entry and zero unresolved **Needs review** entries:

```markdown
## Final report

- [x/ ] Existing mesh CRs converted to HTTPRoute CRs
- [x/ ] StatefulSession / LoadBalance CRs converted to DestinationRule CRs
- [x/ ] Flagged features from Step 1.1 resolved
- [x/ ] Mesh-aware libraries replace old route-posting libraries
- [x/ ] SERVICE_MESH_TYPE set in Helm values / Deployment
- [x/ ] Maven plugin added and local build passes (Java only)
- [x/ ] HTTPRoute CRs generated from route registration code
- [x/ ] Routes with path variables cut to PathPrefix, forbidden routes migrated to AuthorizationPolicy DENY, and every route conflict / forbidden-route question answered
- [x/ ] All HTTPRoute CRs wrapped in the Istio conditional
- [x/ ] HTTPRoutes scanned for duplicate rules (same parent + equal match)
- [x/ ] Imperative control-plane API calls flagged for review

Open items (require user review):
- <list all remaining "Needs review" entries from .mesh-migration/MIGRATION_LOG.md>
```

Close with a plain-language summary telling the user:

1. **What was applied automatically** (reference the Done section count).
2. **What was skipped and why** (reference the Skipped section).
3. **What requires careful human review before merging** — enumerate every
   remaining **Needs review** entry from `.mesh-migration/MIGRATION_LOG.md`
   (sourced from sub-skill `needsReview:` / `# ⚠ MANUAL REVIEW` / `unresolved:`
   items).
4. The recommended validation commands the user should run locally before pushing:

   ```bash
   # Must return at least one HTTPRoute or Gateway line
   helm template <chart> --set SERVICE_MESH_TYPE=Istio \
     | grep -E 'kind: (HTTPRoute|Gateway)'

   # Must return nothing — HTTPRoutes must not leak under Core mode
   helm template <chart> --set SERVICE_MESH_TYPE=Core \
     | grep 'kind: HTTPRoute'
   ```

---


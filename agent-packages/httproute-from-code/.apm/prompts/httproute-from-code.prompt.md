---
description: Generate Gateway API HTTPRoute CRs from Go/Java route-registration code.
argument-hint: <path to file or directory> [backendRefName] [backendRefPort]
---

Apply the `httproute-from-code` skill to generate Gateway API HTTPRoute CRs from
route-registration code.

Source path (and optional backendRefName / backendRefPort): $ARGUMENTS

If no path was given above, ask which file or directory to scan.

Follow the skill exactly:

1. Scan the Go (`*.go`) and Java (`*.java`) files at the given path for
   route-registration definitions (`routeregistration.Route` / `RouteEntry`).
2. Use the provided `backendRefName` / `backendRefPort` / `routeLabels` verbatim;
   if not provided, propose the defaults (`{{ .Values.DEPLOYMENT_RESOURCE_NAME }}`
   / `8080` and the default label set) and confirm before generating.
3. Group routes by type and emit one HTTPRoute CR per type to
   `helm-templates/<service>/templates/source-code-httproutes.yaml`.
4. Sort each HTTPRoute's `rules[]` by path specificity.
5. Print the summary table and resolve any errors before committing the generated
   file.

# public-gateway-authz-policy — evaluation cases

The declarative fixture checks candidate extraction. These cases check the policy skill itself.
Each one starts from a nonempty inventory (`collected: true`, `chartPath` equal to the chart under
test) so an empty-inventory shortcut cannot hide a broken guard.

Inventory used by cases 1–3:

```yaml
chartPath: helm-templates/billing
collected: true
legacyPublic:
  - from: /api/v1/billing/invoices
    matchType: prefix
    gateways: [public-gateway-service]
forbidden:
  - from: /api/v1/billing/invoices
    methods: [DELETE]
    gateways: [public-gateway-service]
  - from: /api/v1/billing/invoices
    methods: [PUT]
    gateways: [public-gateway-service]
  - from: /api/v1/billing/invoices/bulk
    methods: [GET]
    gateways: [public-gateway-service]
deny:
  - from: /api/v1/billing/invoices/internal
    gateways: [public-gateway-service]
  - from: /api/v1/billing/invoices
    gateways: [egress-gateway]
```

| # | Invocation | Expected |
|---|---|---|
| 1 | `generate` omitted or `false` | Report is written (`generated: false`, `fileGenerated: null`). No `public-gateway-authorization-policy.yaml` is created. `.gitignore` may gain `.mesh-migration/` on a standalone run. |
| 2 | `generate: true` and the chart already has an AuthorizationPolicy targeting `public-gateway` | The existing policy file is unchanged. `existingPolicy` is set, `generated: false`, `fileGenerated: null`. |
| 3 | `generate: true`, no existing policy | One ALLOW exception for `/api/v1/billing/invoices` with `notMethods: [DELETE, PUT]`, not two exceptions that each allow the other method. The `/bulk` exception uses `notMethods: [DELETE, PUT, GET]` and the parent exception carves `/bulk` out with `notPaths`. The public DENY lists only `/api/v1/billing/invoices/internal`. The egress deny is reported against `egress-gateway` and is absent from the draft. |
| 4 | Same as 3, but replace the two invoice method entries with one entry that has no `methods` and one `methods: [GET]` on the same path | No ALLOW exception for that path. `needsReview` says the overlap cannot be expressed. The draft must not allow GET. |
| 5 | Report files exist for a different `chartPath`, and no `accessControl` input is passed | `inventory: rejected`, `status: failed`, no policy file. The skill does not treat the other chart's rules as this chart's candidates. |
| 6 | Producer report has no `accessControl.collected: true` (older schema-1 report) | Same rejection as case 5. An omitted block is not an empty inventory. |

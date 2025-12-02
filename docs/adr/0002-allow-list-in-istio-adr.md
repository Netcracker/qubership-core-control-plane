# Allow-List for Anonymous Routes in Istio Ambient Mesh

## Status
**Proposed**

#### Date
*To be defined upon acceptance (ISO 8601)*  
Decision not yet accepted; date will be added later.

#### Owner
[kichasov](https://github.com/kichasov)

#### Participants and Approvers
- Security Team
- Cloud-Core Team  
 [kichasov](https://github.com/kichasov)
 [lis0x90](https://github.com/lis0x90)
 [Ksiona](https://github.com/Ksiona)
 [alagishev](https://github.com/alagishev)
 [iglin](https://github.com/iglin)
 [alsergs](https://github.com/alsergs)

#### Related ADRs
[0001-in-house-service-mesh-replacement](https://github.com/Netcracker/qubership-core-control-plane/blob/main/docs/adr/0001-in-house-service-mesh-replacement.md)

---

## Context

By default, Public and Private gateways in our environment allow anonymous requests to pass through to internally exposed APIs. This behavior unintentionally exposes APIs and increases the overall attack surface.

The goal is to block anonymous requests by default and explicitly define a centralized allow-list of routes that can be accessed without authentication. The feature applies only to **Istio Ambient Mesh**. Cloud-Core Service Mesh is out of scope.

Additional contextual forces:

- Existing customer solutions may rely on anonymous endpoints; enabling blocking by default may break compatibility.
- Istio's RBAC model applies DENY rules first, making distributed allow-listing technically complex.
- Some clients interpret HTTP 401 as a token-retrieval trigger; Istio RBAC uses HTTP 403 for DENY conditions.
- The security team needs a standardized and maintainable mechanism to collect anonymous routes.

Reference Ticket:  
https://github.com/orgs/Netcracker/projects/11/views/8?pane=issue&itemId=139302244

---

## Decision

We will disable anonymous requests by default at the Public and Private gateways in Istio Ambient Mesh.  
We will introduce a **centralized allow-list**, maintained by the Security Team, that defines which routes can be accessed anonymously.

The centralized allow-list will be implemented using an **Istio AuthorizationPolicy** with a DENY action, blocking all anonymous requests except those matching explicitly allowed paths (supports glob patterns).

This policy will be applied at the gateway level (Public and/or Private), ensuring anonymous traffic is blocked as early as possible.

OpenAPI specifications will be used by all microservices to mark anonymous endpoints explicitly. APIHUB will automatically collect this data and provide an anonymous-endpoint report to support allow-list generation.

---

### Justification

- **Security Improvement:** Reduces the attack surface by enforcing explicit control over unauthenticated traffic.
- **Operational Clarity:** A single, centrally managed allow-list avoids fragmented and conflicting configurations.
- **Technical Constraints:** Istio Ambient Mesh evaluates DENY rules first; a distributed model would require complex policy aggregation and is therefore not feasible.
- **Industry Best Practices:**
  - Zero-trust principles: deny by default, explicitly allow exceptions.
  - Early filtering: apply access policies at the edge gateway, not downstream.
  - Consistency: apply security schemes in OpenAPI to document intent and ensure alignment between implementation and specifications.

Alternatives Considered:
- **Distributed allow-list per microservice:**
  - Rejected due to Istio RBAC evaluation behavior and high complexity.
- **Relying on downstream services for access control:**
  - Rejected because gateway-level filtering is more secure and consistent.

---

## Consequences

### Positive
- Stronger protection against unintended API exposure.
- Uniform enforcement of authentication requirements across mesh-exposed endpoints.
- Alignment with zero-trust, defense-in-depth principles.
- Easier auditing and reporting via APIHUB.

### Negative
- Clients relying on HTTP 401 token-retrieval semantics may break and must be updated.
- Projects with legacy anonymous endpoints must contribute accurate allow-list entries.
- Incorrect allow-lists may temporarily block legitimate traffic.

### Neutral
- The solution is not enabled by default to avoid disrupting existing projects.
- Applies only to Istio Ambient Mesh, not Cloud-Core Service Mesh.

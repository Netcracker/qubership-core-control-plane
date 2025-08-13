# Replacement of In-House Service Mesh with Istio Ambient Mesh

## Status
Accepted

#### Date
2025-05-28

#### Owner
[Sergei Aleksandrov](https://github.com/alsergs)

#### Participants and Approvers
- [Sergey Lisovoy](https://github.com/lis0x90)
- [Aleksandr Iglin](https://github.com/iglin)
- [Nelia Loginova](https://github.com/Neliia)
- Viktor Soloviev
- [Segey Pankratov](https://github.com/pankratovsa)
- [Egor Budrin](https://github.com/egbu)
- Andrei Kolchanov

#### Related ADRs
\-

#### Notes
Recommended to revisit migration strategy

## Context
The current in-house service mesh implementation provides basic L4/L7 traffic management. However,

- it requires significant engineering effort to maintain and evolve
- uses own configuration model, different from industry standards
- lack of security capabilities - e.g. no mTLS encryption of mesh traffic

## Decision
We will replace the current in-house service mesh with Istio Ambient Mesh, implement migration tools, strategy and how-tos for existing configurations.

With the introduction of Istio Ambient Mesh, a new dataplane mode with a sidecarless architecture, there is an opportunity to simplify the mesh architecture, reduce operational overhead, and align with a CNCF-backed standard.

### Justification
**Maintainability**:
- Reduces the internal code surface and aligns us with a widely adopted CNCF project.
- Better long-term support and community backing from the Istio and Envoy ecosystems

**Industry alignment**:
- Following best practices by adopting a well-supported open standard with a growing ecosystem.
- Gateway API specification for basic mesh configurations

**Security**:
- Ambient Mesh supports strong zero-trust primitives with zTunnel and optional L7 enforcement via Waypoint proxies (envoy).
- Out-of-the-box mTLS traffic encryption.

**Observability**: Istio Ambient provides robust telemetry out-of-the-box.


#### Alternatives considered:
- Continue evolving the in-house mesh: Rejected due to maintenance cost and lack of community support.
- Migrate to Cilium: Rejected after implementation of test scenario in Istio and Cilium

#### Retrospective
Long list: selected from CNCF landscape - in categories 'Service mesh' and 'API Gateway’, rated by CNCF status, GitHub stars, Last Release Date, Gateway API conformance
Short-list: rated by FR's and NFR's compliance
Istio and Cilium were picked from short list for tests 


## Consequences
**Positive**:
- New feature: out-of-the-box mTLS traffic encryption
- Opportunity to enable rich traffic management features, provided by Istio
- Easier onboarding for new services and developers due to well-documented product
- Ability to hire expertise from market
- Reduced costs for support

**Negative**:
- Complicated migration procedure requires temporary effort
- Required retraining - troubleshooting, guides, bug fixing 
- Temporary resource overhead for parallel mesh working (doubled envoy instances for some gateways)
- Increased latency for backward compatibility cases (extra hop for fallback route)

**Neutral**:
Mesh features and service discovery remain functionally similar
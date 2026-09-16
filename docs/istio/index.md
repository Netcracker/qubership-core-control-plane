# Istio Ambient Mesh Documentation

This section covers Control Plane's integration with Istio Ambient Mesh.

## Contents

- [Istio for Developers](./development/istio-for-devs.md) - Overview of Istio Ambient Mesh architecture, deployment, traffic flow, and troubleshooting guidance for developers.
- [Service Mesh Migration Guide: Cloud-Core Mesh → Istio](./migration-guide/core-mesh-to-istio-migration-guide.md) - Step-by-step guide for migrating from legacy Cloud Core Service Mesh to Istio Ambient Mesh.
- [Egress Routing](./egress-routing.md) - How services reach addresses outside the cluster through
  `egress-gateway`: the path-based request flow, the resources involved, TLS origination, and where
  Cloud-Core Mesh and Istio behave differently.
- [Known Issues](./known-issues.md) - Known behavioral differences and limitations when running on Istio Ambient Mesh.

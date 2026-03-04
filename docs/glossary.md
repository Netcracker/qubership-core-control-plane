This Glossary explains the Control Plane basic terms.

## [Version](#version)

Version is a characteristic/label of an application bundle. It is not the same version which was set by Builder. This characteristic binds to a bundle by the Deployment Job, and only you can use the job in blue-green Deployment mode.
Version in source code is represented by [DeploymentVersion](./api/control-plane-api.md#deploymentversion).
Version can be of several stages: [Active](#active), [Candidate](#candidate), [Legacy](#legacy), [Archived](#archived). Depending on a stage, gateways and facades perform different routing strategies. Example of version values: 'v1', 'v2', 'v3', etc.

## [Active](#active)

The entire user HTTP traffic directs to the micro-services of application bundle which was labeled by Active version.

## [Candidate](#candidate)

When your application bundle is labeled as Candidate, that means that the only user traffic with header 'x-version: <label>' directs to micro-services of this bundle.
For example, your application bundle was labeled during deploy as 'v2' and its status is Candidate. In this case the only requests with header 'x-version: v2' will route to micro-services of  'v2' bundle.

## [Legacy](#legacy)

There are the same rules as those at [Candidate](#candidate).

## [Archived](#archived)

When version of bundle becomes Archived the micro-services will finish taking part in routing.

## [Namespace](#namespace)

# Service Mesh

The following articles and sources describe Service Mesh and it's usage in BlueGreen and Sandbox mode.

* [Service Mesh Development guide](./development-guide.md#service-mesh-development-guide)
    * [Introduction](./development-guide.md#introduction)
    * [Gateway types](./development-guide.md#gateway-types)
    * [Blue-Green](#blue-green)
      * [Blue-Green overview](../bluegreen.md)
      * [Blue-Green prerequisites](./development-guide.md#blue-green-prerequisites)
        * [Parameters](./development-guide.md#parameters)
        * [Deployment configuration](./development-guide.md#deployment-configuration)
    * [Routes registration](./development-guide.md#routes-registration)
        * [Bests practices](./development-guide.md#best-practices)
            * [Always communicate through gateway](./development-guide.md#always-communicate-through-gateway)
            * [Routes naming rules](./development-guide.md#routes-naming-rules)
        * [Routing models](./development-guide.md#routing-models)
        * [Routes registration using configuration files](./development-guide.md#routes-registration-using-configuration-files)
        * [Routes registration using Core Libraries](./development-guide.md#routes-registration-using-core-libraries)
            * [Quarkus Extension](./development-guide.md#quarkus-extension)
            * [Thin Java Libraries 3.X](./development-guide.md#thin-java-libraries-3x)
            * [Reactive Java Library](./development-guide.md#reactive-java-library)
            * [Pure Java](./development-guide.md#pure-java)
            * [Go Route Registration](./development-guide.md#go-route-registration)
            * [Go Microservice Core](./development-guide.md#go-microservice-core)
        * [Routes registration using REST API](./development-guide.md#routes-registration-using-rest-api)
        * [Control Plane Command Line Interface (CLI)](./development-guide.md#control-plane-command-line-interface-cli)
    * [Routes deletion](./routes-deletion-guide.md)
    * [Set route timeout guide](./set-route-timeout-guide.md)
    * [Load balance and sticky session configuration](./development-guide.md#load-balance-and-sticky-session-configuration)
    * [Rate Limiting in Gateways](./rate-limiting.md)
    * [Connections Limiting in Gateways](./max-connections-limit.md)
    * [TCP keepalive for gateway upstream connections](./upstream-connection-tcp-keepalive.md)
    * [Lua Filters](./lua-filtering.md)
    * [Forbid Routes on Public or Private Gateway](./forbid-routes-on-pgw.md)
    * [Custom Ingress Gateways](./ingress-gateways.md)
    * [How to set gateway CORS parameters](TODO OS)
    * [Sample configurations](./development-guide.md#sample-configurations)
        * [Register Public and Private routes](./development-guide.md#register-public-and-private-routes)
        * [Support BlueGreen and add Facade gateway](./development-guide.md#support-bluegreen-and-add-facade-gateway)
        * [Move from Facade gateway to Composite (Mesh) gateway](./development-guide.md#move-from-facade-gateway-to-composite-mesh-gateway)
        * [Move from Composite gateway to Facade gateway](./development-guide.md#move-from-composite-gateway-to-facade-gateway)
        * [Egress gateway](./development-guide.md#egress-gateway)
        * [gRPC routes](./development-guide.md#grpc-routes)
        * [Custom ports for facade and composite gateways](./development-guide.md#custom-ports-for-facade-composite-gateways)
        * [TLS support in microservice](./development-guide.md#tls-support-in-microservice)
* Operations guide
    * how to deploy and operate (promote/rollback) application in BlueGreen mode [BlueGreen](TODO OS)
    * how to deploy application in Sandbox mode [Sandbox](TODO OS)
* [Microservice/Application migration on BlueGreen model](./bluegreen-migration-guide.md)
* Testing [BlueGreen Testing](TODO OS)
* Troubleshooting [Service Mesh troubleshooting guides](TODO OS)
* Performance tests results [Service Mesh Performance Testing Model](TODO OS)
* Installation 
    * CloudCore 6.x requires additional operations to be performed - to prepare cloud https://github.com/Netcracker/qubership-core-facade-operator/tree/main/docs/prerequisites.md 
* Detailed design of components:
    * control-plane https://github.com/Netcracker/qubership-core-control-plane
    * facade-operator https://github.com/Netcracker/qubership-core-facade-operator
    * gateway https://github.com/Netcracker/qubership-core-ingress-gateway
    * Detailed design of BlueGreen mode https://github.com/Netcracker/qubership-core-control-plane/tree/main/docs/bluegreen.adoc#bluegreen

# Microservice/Application migration on BlueGreen model

This document covers steps that need to be performed when migrating microservice on BlueGreen model. 

* [Step 1. Support Parameters provided by Deployer](#step-1-support-parameters-provided-by-deployer)
* [Step 2. Prepare deployment-configuration file](#step-2-prepare-deployment-configuration-file)
* [Step 3. Configure routes registration](#step-3-configure-routes-registration)
* [Step 4. Configure Load Balance (Optional)](#step-4-configure-load-balance-optional)

## Step 1. Support Parameters provided by Deployer

Check carefully that you use these parameters correctly in your template.json, helm charts or any deploy scripts. 

* `SERVICE_NAME` - family service name (ex. `trace-service`)
* `DEPLOYMENT_RESOURCE_NAME` - versioned service name (ex. `trace-service-v2`)
* `DEPLOYMENT_VERSION` - deployment version (ex. `v2`)

So you should use `DEPLOYMENT_RESOURCE_NAME` where you reference to specific service (ex. deployment`s `.metadata.name`) to support BlueGreen model. 

## Step 2. Prepare deployment-configuration file

Deployment-configuration file contains deploy options for your microservice. Using deployment-configuration file you can configure:

1. Generation of named gateway. 
    > :information_source: Despite the amount of different gateway roles, technically all the gateways (except public, private and internal) deployed the same way. So from the deployment-configuration point of view there is no difference between configuring facade, composite or egress gateway - they all are **named gateways** in terms of deployment-configuration file.
2. Setting generated gateway HW resources. 
3. Enabling BlueGreen support. 
    > :warning: Important: All microservices within application should support bluegreen, otherwise BlueGreen deployment will be prohibited for application by deployer.  
4. Status Condition Support for Custom Resource. 

You should place that file to your microservice repository by path `<your-repository-root>/openshift/deployment-configuration.<json or yaml>` 
or `<your-repository-root>/deployments/deployment-configuration.<json or yaml>`.

In the simplest cases it is enough to enable bluegreen support and to set generated gateway name. 

**Facade** gateway should have the same name as microservice family (value from `SERVICE_NAME` env), 
while for **Composite** gateway we can pick any name, it just must be a valid kubernetes service name and must not collide with other service names in project.

E.g. deployment-configuration.yaml for facade gateway generation could look like this: 
```yaml
deployOptions:
  generateFacadeGateway: true
  bluegreen: true
  generateNamedGateway: "${ENV_SERVICE_NAME}"
```

## Step 3. Configure routes registration

There are two ways to automate microservice routes registration procedure: by using configuration files or by using core libraries. 

Please refer to development guide section that suits your needs: 
* [Routes registration using configuration files](./development-guide.md#routes-registration-using-configuration-files)
* [Routes registration using Core Libraries](./development-guide.md#routes-registration-using-core-libraries)
    * [Quarkus Extension](./development-guide.md#quarkus-extension)
    * [Thin Java Libraries 3.X](./development-guide.md#thin-java-libraries-3x)
    * [Reactive Java Library](./development-guide.md#reactive-java-library)
    * [Pure Java](./development-guide.md#pure-java)
    * [Go Microservice Core](./development-guide.md#go-microservice-core)
    
## Step 4. Configure Load Balance (Optional)

Sticky session can be set up by providing load balance configuration in your routes-configuration file. It is implemented by configuring envoy to use [Ring hash](https://www.envoyproxy.io/docs/envoy/v1.13.0/intro/arch_overview/upstream/load_balancing/load_balancers#ring-hash) algorithm based on [Consistent Hashing](https://en.wikipedia.org/wiki/Consistent_hashing).

Consistent hash provides guaranties that balancing model is the same for equal settings of hashing, so you need to set up hashing policies for each cluster that needs to support sticky session. 

Mode of load balancing applies only to entire cluster. That means what you can't switch off and switch on for different version(bg) of a service. It is because each version(bg) of service belongs to the cluster. Each version of services is grouped by cluster.

Below the example of how can you extend your **routes-configuration.yaml** with LoadBalance configuration section: 

```yaml

# here can be routes configurations for other nodeGroups

---
APIVersion: nc.core.mesh/v3
kind: LoadBalance
spec:
  cluster: "${ENV_SERVICE_NAME}"
  version: "${ENV_DEPLOYMENT_VERSION}"
  endpoint: http://${ENV_DEPLOYMENT_RESOURCE_NAME}:8080
  policies:
    - header:
        headerName: "BID"
    - cookie:
        name: "JSESSIONID"
        ttl: 0
```

Full list of available hash policy settings: 
```yaml
- header:
    headerName: "BID"
- cookie:
    name: "JSESSIONID"
    path: "/mypath"
    ttl: 0
```

Load Balancing works only with Headless service (in term k8s). Envoy must read DNS record and know about each pod of balanced service. If you use ClusterIP service then your service is balanced by cloud balancer.

You can read more about hashing policy settings in [envoy docs](https://www.envoyproxy.io/docs/envoy/v1.13.0/api-v2/api/v2/route/route_components.proto#envoy-api-msg-route-routeaction-hashpolicy). 

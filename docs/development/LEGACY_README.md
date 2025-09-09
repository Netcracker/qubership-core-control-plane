[//]: ![logo](img/logo.png)
# Control Plane Overview

This documentation consists of the following topics:

* [Features](#features)
    * [Routing table management](#routing-table-management)
    * [Routes registration](#routes-registration)
* [REST API](#rest-api)
* [Blue-Green](#blue-green)
* [Service Mesh](/docs/mesh)
* [Watching versions](/docs/watch.md)
* [Runtime processes](#runtime-processes)
* [Tools](#tools)
    * [Control Plane CLI](#control-plane-cli)
* [Helpful links](#helpful-links)

## Features

* routing table management
* routes registration
* preparation envoy proxy configuration
* routes storage
* central link of Blue-Green mode

### Routing table management

Routing table contains mapping upstream (incoming) request parameter on downstream (target) service version.  
The role of routing table is to define a microservice, that will serve a particular request, by the request path and version.    
Version is provided in “x-version” header.  
Routing table has version by default - it will be used when no version is specified, or when no version is found.

![routing_table](/docs/img/routing_table.png)

A very simplified representation of routing table contains the following columns:
* **Target URL**. Route published on gateway. There could be several routes at a time, and `/` wildcard is used to register any possible route of target microservice.
* **Blue-Green version**. Blue-Green version of application. Routing table compares this value with the value in header.  One version (that corresponds to Active version) is meant the default and requests w/o header will be routed to this version.
* **Cluster**. Microservice family name. Gateway could serve multiple families at a time, mostly it is common for public/private gateways and in cases when one façade gateway serves several families.
* **Endpoint**. URL of service to be called. Could be definite endpoint (as `http://a-v1:8080/api/getOrder`) or microservice root (`http://a-v1:8080`).

Control Plane contains all routing tables of all gateways and distributes changes.

### Routes registration

##### !!! Routing modes were added at control-plane in order to separate two functions: namespace routing (sandboxes) and blue-green. Accordingly, if you have registered at least one route with a namespace different from the one where control-plane is located, the mode is set to 'namespace' and you will not be able to register a route with a version (for blue-green deployment). The reverse situation is also true. You can change the mode by deleting all the routes that are typical for the activated mode (for example, by deleting all the routes with namespace, you activate the intermediate-simple mode).

Control Plane provides REST API and CLI to register/unregister routes and to follow Blue-Green lifecycle (promote, rollback).
The picture below shows how the Routes Registration API and routing table are related.  
![routing_diagram](/docs/img/routing_diagram.png)

Control Plane API is provided in two flavors - REST and CLI. CLI proxies REST calls, so they are functionally identical.     
Detailed information about Registration API is presented in the respective REST API section.  
Show more about Control Plane CLI in [Control Plane CLI](#control-plane-cli).

##  REST API

Control Plane REST API provides an easy way to registration routes for your microservice.  
This API is supported in the cloud-core library, where you can use annotation to automatically register routes during the deployment of a microservice.  

Control Plane REST API  also provides the functionality for supporting the Blue-Green. The detailed information about the Blue-Green is presented in the next section.

##  Blue-Green

In Blue-Green mode, an application could exist in several versions at a time. It means that there should exist several versions of the same microservice.  
Kubernetes does not allow such a behavior, so it is emulated: each microservice version is installed in deployments having different names as version suffix is added.

Example:  
**_quote-storage_** microservice will be deployed with the names **_quote-storage-v1_** , **_quote-storage-v2_** etc.

Version is incremental sequence that is automatically assigned to a candidate, it does not have any relation to the application version (such as `master-20191210.122037-21-RELEASE`).

The same application version deployed to different namespaces will have different versions.  
Group of different versions of one microservice is called Family.  
Gateway acts as a facade over the versions and provides a single access point to multiple versions through one entry point - Gateway service.  
Thus, an application has different deployment architecture when deployed in rolling and Blue-Green modes:
* In rolling mode a microservice has no versions. One deployment config, one service.
* In Blue-Green mode several transformations are performed:
    * Microservice (deployment config and service name) obtains version suffix automatically added by DevOps procedures.
    * (optionally) Facade gateway is added – a dedicated gateway that will serve (and will belong to) the particular family.
    * (optionally) Service name without suffix is given to facade gateway.

In the image below you can see the communication through the gateways in Blue-Green mode.

![Blue-Green communication](/docs/img/bluegreen-arch.png)

See the detailed information about the Blue-Green operation in the following section: [Blue-Green](/docs/bluegreen.md).

## Runtime processes

![Timeline diagram](/docs/img/time-line-diagram.png)

### Load common config

***When:*** Once after start application

Process sends a common set of parameters to envoy proxy (to each gateway). The parameters set includes listeners, clusters, base routes.
Hardcoded settings are also written to the database. If some additional settings are present in the database they also will be sent to envoy.
Processes and parameters are described in the following classes and packages:
* `org.qubership.cloud.controlplane.configs.CommonConfiguration`
* `org.qubership.cloud.controlplane.grpc.manager.*`

### Migration

***When:*** Once after start application

The process reads old description of the routes from Config Server, translates them to envoy settings and writes them down to the database and envoy proxy.
It can be run manually. Call `Set routes migration flag` endpoint with `false` parameter and restart application.

### Cleanup

***When:*** Every day at 2 a.m.

The process removes the routes which are not up-to-date or were created for local development. It removes them from the database and updates each gateway.

### Tenant-manager watch

***When:*** All the time

The process listens to Tenant Manager microservice and catches the events which describe a new tenant creation. This is done for appending namespace information to headers in requests passed through envoy proxy.

## Environment (deploy) parameters

### Allow Origin

`GATEWAYS_ALLOWED_ORIGIN` environment variable describes the domains allowed for routing into cloud.

To allow requests from any origin it should be set to `*`.

Default value: `"*.qubership.org:*, *.qubership.cloud:*"`

### Route timeout

`TIMEOUT` environment variable specifies the timeout for a route in milliseconds. This spans between the point at which the entire downstream request (i.e. end-of-stream) has been processed and when the upstream response has been completely processed. This timeout includes all retries.

Default value: `60000`

##  Tools

### Control Plane CLI

Control Plane CLI provides a better way to interact with Control Plane microservice using command line style.  
This CLI is just a tiny wrapper over Control Plane REST interface.  

## Helpful links
* [Facade Operator](https://github.com/Netcracker/qubership-core-facade-operator)
* [DBaaS](https://github.com/Netcracker/qubership-dbaas)
* [gRPC](https://grpc.io/)
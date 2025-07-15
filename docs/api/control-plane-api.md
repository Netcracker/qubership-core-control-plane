This documentation describes Control Plane API.

# Overview

API includes registration of routes, blue-green features.


## Version information

*Version* : 1.0


## URI scheme

*Protocol* : http
*Host* : control-plane:8080  
*BasePath* : /

## Table of contents
[[_TOC_]]

# API v1
## Migration Management API

Service Controller


### Get migration flag

```
GET /api/v1/control-plane/system/migration-done
```


#### Description

Get value of migration flag.


#### Responses

| HTTP Code | Description | Schema                            |
|:----------|:------------|:----------------------------------|
| **200**   | OK          | [ResponseEntity](#responseentity) |


#### Produces

* `\*/*`
* `text/plain`


#### Example HTTP request

##### Request path

```
/api/v1/control-plane/system/migration-done
```


#### Example HTTP response

##### Response 200

```json
{
  "body" : "object",
  "statusCode" : "string",
  "statusCodeValue" : 0
}
```

### Set migration flag

```
POST /api/v1/control-plane/system/migration-done/{value}
```


#### Description

Set is done flag. If true migration job will be stopped.


#### Parameters

| Type     | Name                      | Description | Schema             |
|:---------|:--------------------------|:------------|:-------------------|
| **Path** | **value**  <br>*required* | value       | enum (true, false) |


#### Responses

| HTTP Code | Description | Schema                            |
|:----------|:------------|:----------------------------------|
| **200**   | OK          | [ResponseEntity](#responseentity) |


#### Consumes

* `application/json`


#### Produces

* `\*/*`


#### Example HTTP request

##### Request path

```
/api/v1/control-plane/system/migration-done/true
```


#### Example HTTP response

##### Response 200

```json
{
  "body" : "object",
  "statusCode" : "string",
  "statusCodeValue" : 0
}
```
## Route Management - Version 1

Route Controller


### Get all clusters

```
GET /api/v1/routes/clusters
```


#### Responses

| HTTP Code | Description | Schema                            |
|:----------|:------------|:----------------------------------|
| **200**   | OK          | [ResponseEntity](#responseentity) |


#### Produces

* `\*/*`


#### Example HTTP request

##### Request path

```
/api/v1/routes/clusters
```


#### Example HTTP response

##### Response 200

```json
{
  "body" : "object",
  "statusCode" : "string",
  "statusCodeValue" : 0
}
```


### Delete cluster by id

```
DELETE /api/v1/routes/clusters/{id}
```


#### Parameters

| Type     | Name                   | Description         | Schema          |
|:---------|:-----------------------|:--------------------|:----------------|
| **Path** | **id**  <br>*optional* | Cluster identifier. | integer (int64) |


#### Responses

| HTTP Code | Description | Schema                            |
|:----------|:------------|:----------------------------------|
| **204**   | No Content  | [ResponseEntity](#responseentity) |


#### Produces

* `\*/*`


#### Example HTTP request

##### Request path

```
/api/v1/routes/clusters/44
```


#### Example HTTP response

##### Response 200

```json
{
  "body" : "object",
  "statusCode" : "string",
  "statusCodeValue" : 0
}
```


### Get all routes configurations

```
GET /api/v1/routes/route-configs
```


#### Responses

| HTTP Code | Description | Schema                            |
|:----------|:------------|:----------------------------------|
| **200**   | OK          | [ResponseEntity](#responseentity) |


#### Produces

* `\*/*`


#### Example HTTP request

##### Request path

```
/api/v1/routes/route-configs
```


#### Example HTTP response

##### Response 200

```json
{
  "body" : "object",
  "statusCode" : "string",
  "statusCodeValue" : 0
}
```


### Get all envoy node-groups

```
GET /api/v1/routes/node-groups
```


#### Responses

| HTTP Code | Description | Schema                            |
|:----------|:------------|:----------------------------------|
| **200**   | OK          | [ResponseEntity](#responseentity) |


#### Produces

* `\*/*`


#### Example HTTP request

##### Request path

```
/api/v1/routes/node-groups
```


#### Example HTTP response

##### Response 200

```json
{
  "body" : "object",
  "statusCode" : "string",
  "statusCodeValue" : 0
}
```


### Get all listeners

```
GET /api/v1/routes/listeners
```


#### Responses

| HTTP Code | Description | Schema                            |
|:----------|:------------|:----------------------------------|
| **200**   | OK          | [ResponseEntity](#responseentity) |


#### Produces

* `\*/*`


#### Example HTTP request

##### Request path

```
/api/v1/routes/listeners
```


#### Example HTTP response

##### Response 200

```json
{
  "body" : "object",
  "statusCode" : "string",
  "statusCodeValue" : 0
}
```


### Create/Update routes for node-group

```
POST /api/v1/routes/{nodeGroup}
```


#### Parameters

| Type     | Name                          | Description              | Schema |
|:---------|:------------------------------|:-------------------------|:-------|
| **Path** | **nodeGroup**  <br>*optional* | Name of envoy node-group | string |


#### Body parameter

Description of routes to be inserted

*Name* : request  
*Flags* : optional  
*Type* : [RouteEntityRequest](#routeentityrequest)


#### Responses

| HTTP Code | Description | Schema                                                                      |
|:----------|:------------|:----------------------------------------------------------------------------|
| **200**   | OK          | DeferredResult«ResponseEntity«object»» |


#### Consumes

* `application/json`


#### Produces

* `\*/*`


#### Example HTTP request

##### Request path

```
/api/v1/routes/public-gateway-service
```


##### Request body

```json
{
  "allowed" : true,
  "microserviceUrl" : "string",
  "routes" : [ {
    "from" : "string",
    "timeout" : 0,
    "to" : "string",
    "namespace" : "string"
  } ]
}
```


#### Example HTTP response

##### Response 200

```json
{
  "result" : "object",
  "setOrExpired" : true
}
```


### Delete routes by node-group

```
DELETE /api/v1/routes/{nodeGroup}
```


#### Parameters

| Type      | Name                          | Description                                                     | Schema |
|:----------|:------------------------------|:----------------------------------------------------------------|:-------|
| **Path**  | **nodeGroup**  <br>*required* | Name of envoy node-group                                        | string |
| **Query** | **from**  <br>*optional*      | Which path handle to route                                      | string |
| **Query** | **namespace**  <br>*optional* | Cloud env namespace. Openshift - project name, K8s - namespace. | string |


#### Responses

| HTTP Code | Description | Schema                            |
|:----------|:------------|:----------------------------------|
| **204**   | No Content  | [ResponseEntity](#responseentity) |


#### Produces

* `\*/*`


#### Example HTTP request

##### Request path

```
/api/v1/routes/public-gateway-service
```


#### Example HTTP response

##### Response 200

```json
{
  "body" : "object",
  "statusCode" : "string",
  "statusCodeValue" : 0
}
```

# API v2
## Blue-green Functionality

Blue Green Controller V 2

### Promote

```
POST /api/v2/control-plane/promote/{version}
```


#### Description

Promotes specified version. Makes version as ACTIVE and previous ACTIVE
becomes LEGACY.


#### Parameters

| Type      | Name                            | Description                      | Schema          |
|:----------|:--------------------------------|:---------------------------------|:----------------|
| **Path**  | **version**  <br>*required*     | version to promote               | string          |
| **Query** | **archiveSize**  <br>*optional* | size of archive being left at CP | integer (int32) |


#### Responses

| HTTP Code | Description | Schema                                            |
|:----------|:------------|:--------------------------------------------------|
| **202**   | Accepted    | < [DeploymentVersion](#deploymentversion) > array |


#### Consumes

* `application/json`


#### Produces

* `application/json;charset=UTF-8`


#### Example HTTP request

##### Request path

```
/api/v2/control-plane/promote/v4
```

#### Example HTTP response

##### Response 202

```json
[ {
  "createdWhen" : "string",
  "stage" : "string",
  "updatedWhen" : "string",
  "version" : "string"
} ]
```

### Rollback

```
POST /api/v2/control-plane/rollback
```


#### Description

Returns state of versions back to state before promoting.


#### Responses

| HTTP Code | Description | Schema                                            |
|:----------|:------------|:--------------------------------------------------|
| **202**   | Accepted    | < [DeploymentVersion](#deploymentversion) > array |


#### Consumes

* `application/json`


#### Produces

* `application/json;charset=UTF-8`


#### Example HTTP request

##### Request path

```
/api/v2/control-plane/rollback
```


#### Example HTTP response

##### Response 202

```json
[ {
  "createdWhen" : "string",
  "stage" : "string",
  "updatedWhen" : "string",
  "version" : "string"
} ]
```

### Get routing mode details

```
GET /api/v2/control-plane/routing/details
```


#### Responses

| HTTP Code | Description | Schema                                    |
|:----------|:------------|:------------------------------------------|
| **200**   | OK          | [RoutingModeDetails](#routingmodedetails) |


#### Produces

* `\*/*`


#### Example HTTP request

##### Request path

```
/api/v2/control-plane/routing/details
```


#### Example HTTP response

##### Response 200

```json
{
  "routeKeys" : [ "string" ],
  "routingMode" : "string"
}
```

### Get all deployment versions

```
GET /api/v2/control-plane/versions
```


#### Responses

| HTTP Code | Description | Schema                                            |
|:----------|:------------|:--------------------------------------------------|
| **200**   | OK          | < [DeploymentVersion](#deploymentversion) > array |


#### Produces

* `\*/*`


#### Example HTTP request

##### Request path

```
/api/v2/control-plane/versions
```


#### Example HTTP response

##### Response 200

```json
[ {
  "createdWhen" : "string",
  "stage" : "string",
  "updatedWhen" : "string",
  "version" : "string"
} ]
```

### Delete version

```
DELETE /api/v2/control-plane/versions/{version}
```


#### Parameters

| Type     | Name                        | Description | Schema |
|:---------|:----------------------------|:------------|:-------|
| **Path** | **version**  <br>*required* | version     | string |


#### Responses

| HTTP Code | Description | Schema     |
|:----------|:------------|:-----------|
| **200**   | OK          | No Content |


#### Produces

* `\*/*`


#### Example HTTP request

##### Request path

```
/api/v2/control-plane/versions/string
```


## Load balancing and sticky session API
### Apply load balance configuration

```
POST /api/v2/control-plane/load-balance
```


#### Body parameter


*Name* : loadBalanceSpec  
*Flags* : required  
*Type* : [LoadBalanceSpec](#loadbalancespec)


#### Responses

| HTTP Code | Description | Schema     |
|:----------|:------------|:-----------|
| **200**   | OK          | No Content |


#### Consumes

* `application/json`


#### Produces

* `\*/*`


#### Example HTTP request

##### Request path

```
/api/v2/control-plane/load-balance
```


##### Request body

```json
{
  "cluster": "test-cluster",
  "endpoint": "trace-service-1:8080",
  "version": "v1",
  "policies": [
    {
      "header": {
        "headerName": "BID"
      },
      "cookie": {
        "name": "JSESSIONID",
        "ttl": 0
      }
    }
  ]
}
```


#### Example HTTP response

##### Response 200

```json

```
## Routes Management - Version 2

Route Controller V 2


### Delete Routes

```
DELETE /api/v2/control-plane/routes
```


#### Body parameter

deleteRequests

*Name* : deleteRequests  
*Flags* : required  
*Type* : < [RouteDeleteRequest](#routedeleterequest) > array


#### Responses

| HTTP Code | Description | Schema                    |
|:----------|:------------|:--------------------------|
| **200**   | OK          | < [Route](#route) > array |


#### Produces

* `\*/*`


#### Example HTTP request

##### Request path

```
/api/v2/control-plane/routes
```


##### Request body

```json
[ {
  "namespace" : "string",
  "routes" : [ {
    "prefix" : "string"
  } ],
  "version" : "string"
} ]
```


#### Example HTTP response

##### Response 200

```json
[ {
  "action" : {
    "clusterName" : "string",
    "hostAutoRewrite" : true,
    "hostRewrite" : "string",
    "pathRewrite" : "string",
    "prefixRewrite" : "string"
  },
  "autoGenerated" : true,
  "deploymentVersion" : {
    "createdWhen" : "string",
    "stage" : "string",
    "updatedWhen" : "string",
    "version" : "string"
  },
  "directResponseAction" : {
    "status" : 0
  },
  "id" : 0,
  "initialDeploymentVersion" : "string",
  "matcher" : {
    "headers" : [ {
      "exactMatch" : "string",
      "id" : 0,
      "name" : "string",
      "version" : 0
    } ],
    "prefix" : "string",
    "regExp" : "string"
  },
  "routeKey" : "string",
  "timeout" : 0,
  "timeoutSeconds" : 0,
  "version" : 0
} ]
```

### Create/Update routes

```
POST /api/v2/control-plane/routes/{nodeGroup}
```


#### Description

Create or Update routes for specified node-group.


#### Parameters

| Type     | Name                          | Description                                 | Schema |
|:---------|:------------------------------|:--------------------------------------------|:-------|
| **Path** | **nodeGroup**  <br>*required* | tells which group of envoys must get routes | string |


#### Body parameter

set of routes and additional info for routing

*Name* : registrationRequest  
*Flags* : required  
*Type* : < [RouteRegistrationRequest](#routeregistrationrequest) > array


#### Responses

| HTTP Code | Description | Schema                            |
|:----------|:------------|:----------------------------------|
| **200**   | OK          | [DeferredResult](#deferredresult) |


#### Consumes

* `application/json;charset=UTF-8`


#### Produces

* `\*/*`


#### Example HTTP request

##### Request path

```
/api/v2/control-plane/routes/string
```


##### Request body

```json
[ {
  "allowed" : true,
  "cluster" : "string",
  "endpoint" : "string",
  "namespace" : "string",
  "routes" : [ {
    "prefix" : "string",
    "prefixRewrite" : "string",
    "headerMatchers": [
    {
      "name": ":method",
      "exactMatch": "POST"
    },
    {
      "name": "magic-header"
    }
    ]
  } ],
  "version" : "string"
} ]
```


#### Example HTTP response

##### Response 200

```json
{
  "result" : "object",
  "setOrExpired" : true
}
```


### Delete routes by node-group

```
DELETE /api/v2/control-plane/routes/{nodeGroup}
```


#### Parameters

| Type     | Name                          | Description | Schema |
|:---------|:------------------------------|:------------|:-------|
| **Path** | **nodeGroup**  <br>*required* | nodeGroup   | string |


#### Body parameter

deleteRequests

*Name* : deleteRequests  
*Flags* : required  
*Type* : < [RouteDeleteRequest](#routedeleterequest) > array


#### Responses

| HTTP Code | Description | Schema                    |
|:----------|:------------|:--------------------------|
| **200**   | OK          | < [Route](#route) > array |


#### Produces

* `\*/*`


#### Example HTTP request

##### Request path

```
/api/v2/control-plane/routes/string
```


##### Request body

```json
[ {
  "namespace" : "string",
  "routes" : [ {
    "prefix" : "string"
  } ],
  "version" : "string"
} ]
```


#### Example HTTP response

##### Response 200

```json
[ {
  "action" : {
    "clusterName" : "string",
    "hostAutoRewrite" : true,
    "hostRewrite" : "string",
    "pathRewrite" : "string",
    "prefixRewrite" : "string"
  },
  "autoGenerated" : true,
  "deploymentVersion" : {
    "createdWhen" : "string",
    "stage" : "string",
    "updatedWhen" : "string",
    "version" : "string"
  },
  "directResponseAction" : {
    "status" : 0
  },
  "id" : 0,
  "initialDeploymentVersion" : "string",
  "matcher" : {
    "headers" : [ {
      "id": 0,
      "name" : "string",
      "version" : 0,
      "exactMatch" : "string",
      "safeRegexMatch" : "string",
      "rangeMatch": {
        "start": 0,
        "end": 0
      },
      "presentMatch" : false,
      "prefixMatch" : "string",
      "suffixMatch" : "string",
      "invertMatch" : false
    } ],
    "prefix" : "string",
    "regExp" : "string"
  },
  "routeKey" : "string",
  "timeout" : 0,
  "timeoutSeconds" : 0,
  "version" : 0
} ]
```


### Delete route by UUID

```
DELETE /api/v2/control-plane/routes/uuid/{uuid}
```


#### Parameters

| Type     | Name                          | Description | Schema |
|:---------|:------------------------------|:------------|:-------|
| **Path** | **uuid**  <br>*required*      | Route UUID  | UUID   |


#### Responses

| HTTP Code | Description | Schema                    |
|:----------|:------------|:--------------------------|
| **200**   | OK          | [Route](#route)           |


#### Produces

* `application/json`


#### Example HTTP request

##### Request path

```
/api/v2/control-plane/routes/uuid/285d3ba5-d024-4c29-a7d8-2eec7e03aafa
```


#### Example HTTP response

##### Response 200

```json
{
  "Id": 182,
  "Uuid": "285d3ba5-d024-4c29-a7d8-2eec7e03aafa",
  "VirtualHostId": 3,
  "VirtualHost": null,
  "RouteKey": "||/api/v4/tenant-manager/openshift||v1",
  "DirectResponseCode": 404,
  "Prefix": "/api/v4/tenant-manager/openshift",
  "Regexp": "",
  "ClusterName": "tenant-manager||tenant-manager||8080",
  "HostRewrite": "tenant-manager:8080",
  "HostAutoRewrite": null,
  "PrefixRewrite": "",
  "PathRewrite": "",
  "Version": 2,
  "Timeout": null,
  "DeploymentVersion": "v1",
  "DeploymentVersionVal": {
    "version": "v1",
    "stage": "ACTIVE",
    "createdWhen": "2020-08-11T09:00:38.841312Z",
    "updatedWhen": "2020-08-11T09:00:38.841312Z"
  },
  "InitialDeploymentVersion": "v1",
  "Autogenerated": false,
  "HeaderMatchers": [],
  "HashPolicies": []
}
```

### Delete endpoints

```http request
DELETE /api/v2/control-plane/endpoints
```

#### Responses

| HTTP Code | Description | Scheme                          |
|:----------|:------------|:--------------------------------|
| **200**   | OK          | < [Endpoint](#Endpoint) > array |
| **400**   | Bad request |                                 |

#### Body parameter

deleteRequests

*Name* : endpointDeleteRequest  
*Flags* : required  
*Type* : < [EndpointDeleteRequest](#EndpointDeleteRequest) > array

#### Example request

Request path

```http request
DELETE /api/v2/control-plane/endpoints
```

Request body

```json
[
  {
    "endpoints": [
      {
        "address": "test-endpoint",
        "port": 8080
      }
    ],
    "version": "v1"
  }
]
```

#### Example response

Response 200

```json
[
  {
    "Id": 11,
    "Address": "test-endpoint",
    "Port": 8080,
    "ClusterId": 11,
    "Cluster": null,
    "DeploymentVersion": "v1",
    "InitialDeploymentVersion": "v1",
    "DeploymentVersionVal": {
      "version": "v1",
      "stage": "ACTIVE",
      "createdWhen": "2021-12-20T11:28:16.906402Z",
      "updatedWhen": "2021-12-20T11:28:16.906402Z"
    },
    "HashPolicies": null,
    "Hostname": "",
    "OrderId": 0
  }
]
```

# API v3
## Blue-green Functionality

### Promote

```
POST /api/v3/control-plane/promote/{version}
```

#### Description

Promotes specified version. Makes version as ACTIVE and previous ACTIVE
becomes LEGACY.


#### Parameters

| Type      | Name                            | Description                      | Schema          |
|:----------|:--------------------------------|:---------------------------------|:----------------|
| **Path**  | **version**  <br>*required*     | version to promote               | string          |
| **Query** | **archiveSize**  <br>*optional* | size of archive being left at CP | integer (int32) |


#### Responses

| HTTP Code | Description | Schema                                            |
|:----------|:------------|:--------------------------------------------------|
| **202**   | Accepted    | < [DeploymentVersion](#deploymentversion) > array |


#### Consumes

* `application/json`


#### Produces

* `application/json;charset=UTF-8`


#### Example HTTP request

##### Request path

```
/api/v3/control-plane/promote/v4
```

#### Example HTTP response

##### Response 202

```json
[ {
  "createdWhen" : "string",
  "stage" : "string",
  "updatedWhen" : "string",
  "version" : "string"
} ]
```

### Rollback

```
POST /api/v3/control-plane/rollback
```

#### Description

Returns state of versions back to state before promoting.


#### Responses

| HTTP Code | Description | Schema                                            |
|:----------|:------------|:--------------------------------------------------|
| **202**   | Accepted    | < [DeploymentVersion](#deploymentversion) > array |


#### Consumes

* `application/json`


#### Produces

* `application/json;charset=UTF-8`

#### Example HTTP request

##### Request path

```
/api/v2/control-plane/rollback
```


#### Example HTTP response

##### Response 202

```json
[ {
  "createdWhen" : "string",
  "stage" : "string",
  "updatedWhen" : "string",
  "version" : "string"
} ]
```

### Get routing mode details

```
GET /api/v3/control-plane/routing/details
```


#### Responses

| HTTP Code | Description | Schema                                    |
|:----------|:------------|:------------------------------------------|
| **200**   | OK          | [RoutingModeDetails](#routingmodedetails) |


#### Produces

* `\*/*`


#### Example HTTP request

##### Request path

```
/api/v3/control-plane/routing/details
```

#### Example HTTP response

##### Response 200

```json
{
  "routeKeys" : [ "string" ],
  "routingMode" : "string"
}
```

### Get all deployment versions

```
GET /api/v3/control-plane/versions
```


#### Responses

| HTTP Code | Description | Schema                                            |
|:----------|:------------|:--------------------------------------------------|
| **200**   | OK          | < [DeploymentVersion](#deploymentversion) > array |


#### Produces

* `\*/*`


#### Example HTTP request

##### Request path

```
/api/v3/control-plane/versions
```


#### Example HTTP response

##### Response 200

```json
[ {
  "createdWhen" : "string",
  "stage" : "string",
  "updatedWhen" : "string",
  "version" : "string"
} ]
```

### Get deployment version for microservice

Provides actual value of b/g version that microservice should set in `X-Version` header by default (when interaction initiated by this microservice itself). 
If returned version value is empty, microservice should not add `X-Version` header at all since it is located in `ACTIVE` b/g version and gateways will resolve proper version for its requests by default. 

```
GET /api/v3/control-plane/versions/microservices/{microservice}
```

#### Path variables

| Name | Description | Schema                                            |
|:----------|:------------|:--------------------------------------------------|
| microservice  | Microservice hostname (including version suffix if microservice supports versioning). Example: `employee-service-v2` | string |


#### Responses

| HTTP Code | Description                                                                             | Schema                                                |
|:----------|:----------------------------------------------------------------------------------------|:------------------------------------------------------|
| **200**   | OK                                                                                      | < [MicroserviceVersion](#microserviceversion) > array |
| **200**   | Not Found (there is no endpoint with the specified host in control-plane configuration) |                                                       |


#### Produces

* `application/json`


#### Example HTTP request

##### Request path

```
/api/v3/control-plane/versions/microservices/employee-service-v2
```


#### Example HTTP response

##### Response 200

```json
{
  "version" : "v2"
}
```

### Delete version

```
DELETE /api/v3/control-plane/versions/{version}
```


#### Parameters

| Type     | Name                        | Description | Schema |
|:---------|:----------------------------|:------------|:-------|
| **Path** | **version**  <br>*required* | version     | string |


#### Responses

| HTTP Code | Description | Schema     |
|:----------|:------------|:-----------|
| **200**   | OK          | No Content |


#### Produces

* `\*/*`


#### Example HTTP request

##### Request path

```
/api/v3/control-plane/versions/string
```

### Register services in Blue-Green version registry

```
POST /api/v3/control-plane/versions/registry
```


#### Request Body: [ServicesVersion](#ServicesVersion)


#### Responses

| HTTP Code | Description | Schema |
|:----------|:------------|:-------|
| **200**   | OK          | json   |
| **400**   | Bad Request | json   |


### Delete services from Blue-Green version registry

```
DELETE /api/v3/control-plane/versions/registry/services
```


#### Request Body: [ServicesVersion](#ServicesVersion)


#### Responses

| HTTP Code | Description | Schema |
|:----------|:------------|:-------|
| **200**   | OK          | json   |
| **400**   | Bad Request | json   |


### Get services versions from Blue-Green version registry

```
GET /api/v3/control-plane/versions/registry
```


#### Request Params

| Name                             | Type        | Description                                                                                                                                                                                                                                                           |
|:---------------------------------|:------------|:----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| serviceName <br> **optional**    | Query Param | Microservice family name without version suffix - as in SERVICE_NAME env variable provided by deployer or in config.xml. Example: `trace-service`                                                                                                                     |
| namespace <br> **optional**      | Query Param | Namespace where microservice is deployed. Only applicable for SANDBOX deploy model. Normally you should not specify this field - control-plane will resolve namespace automatically.                                                                                  |
| version <br> **optional**        | Query Param | Blue-Green version, e.g. `v1`. This parameter is used to filter services based on the actual blue-green version they serve now, not the version in which they were deployed (see `initialVersion`).                                                                   |
| initialVersion <br> **optional** | Query Param | Blue-Green version in which microservice was deployed (value of DEPLOYMENT_VERSION env provided by deployer). Only applicable when coupled with `serviceName` param - such request can be used to determine actual microservice version based on its initial version. |

#### Responses

| HTTP Code | Description | Schema                                            |
|:----------|:------------|:--------------------------------------------------|
| **200**   | OK          | < [VersionInRegistry](#VersionInregistry) > array |
| **400**   | Bad Request | json                                              |


### Apply load balance configuration

```
POST /api/v3/control-plane/load-balance
```


#### Body parameter


*Name* : loadBalanceSpec  
*Flags* : required  
*Type* : [LoadBalanceSpec](#loadbalancespec)


#### Responses

| HTTP Code | Description | Schema     |
|:----------|:------------|:-----------|
| **200**   | OK          | No Content |


#### Consumes

* `application/json`


#### Produces

* `\*/*`


#### Example HTTP request

##### Request path

```
/api/v3/control-plane/load-balance
```


##### Request body

```json
{
  "cluster": "test-cluster",
  "endpoint": "trace-service-1:8080",
  "version": "v1",
  "policies": [
    {
      "header": {
        "headerName": "BID"
      },
      "cookie": {
        "name": "JSESSIONID",
        "ttl": 0
      }
    }
  ]
}
```


#### Example HTTP response

##### Response 200

```json

```




### Apply cookie based stateful session configuration

```
POST/PUT /api/v3/control-plane/load-balance/stateful-session
```


#### Body parameter


*Name* : StatefulSession  
*Flags* : required  
*Type* : [StatefulSession](#statefulsession)


#### Responses

| HTTP Code | Description | Schema     |
|:----------|:------------|:-----------|
| **200**   | OK          | `application/json` |


#### Consumes

* `application/json`


#### Produces

* `application/json`


#### Example HTTP request

##### Request path

```
/api/v3/control-plane/load-balance/stateful-session
```


##### Request body

```json
{
  "gateways": ["internal-gateway-service"],
  "cluster": "test-cluster",
  "port": 8080,
  "version": "v1",
  "enabled": true,
  "cookie": {
        "name": "JSESSIONID",
        "ttl": 0
  }
}
```


#### Example HTTP response

##### Response 200

```json
{ "message": "StatefulSession configuration applied successfully" }
```


### Delete cookie based stateful session configuration

```
POST/PUT/DELETE /api/v3/control-plane/load-balance/stateful-session
```

Request body must contain no `enabled` and `cookie` fields, and then configuration will be cleared. 

#### Body parameter


*Name* : StatefulSession  
*Flags* : required  
*Type* : [StatefulSession](#statefulsession)


#### Responses

| HTTP Code | Description | Schema     |
|:----------|:------------|:-----------|
| **200**   | OK          | `application/json` |


#### Consumes

* `application/json`


#### Produces

* `application/json`


#### Example HTTP request

##### Request path

```
/api/v3/control-plane/load-balance/stateful-session
```


##### Request body

```json
{
  "gateways": ["internal-gateway-service"],
  "cluster": "test-cluster",
  "port": 8080,
  "version": "v1",
}
```


#### Example HTTP response

##### Response 200

```json
{ "message": "StatefulSession configuration applied successfully" }
```


### Get all rate limit configurations

```
GET /api/v3/control-plane/rate-limits
```

Returns all the rate limit configurations. 

#### Responses

| HTTP Code | Description | Schema     |
|:----------|:------------|:-----------|
| **200**   | OK          | list <[RateLimit](#ratelimit)> |


#### Consumes

* `application/json`


#### Produces

* `application/json`


#### Example HTTP request

##### Request path

```
/api/v3/control-plane/rate-limits
```


#### Example HTTP response

##### Response 200

```json
[
  { "name": "rate-limit1", "limitRequestPerSecond": 10, "priority": "PROJECT" },
  { "name": "rate-limit2", "limitRequestPerSecond": 5, "priority": "PRODUCT" }
]
```


### Apply rate limit configuration

```
POST /api/v3/control-plane/rate-limits
```

Applies rate limit configuration: saves or overrides existing configuration, if "limitRequestPerSecond" field is greater then 0, otherwise deletes rate limit configuration with the specified "name" and "priority". 

#### Body parameter


*Name* : RateLimit  
*Flags* : required  
*Type* : [RateLimit](#RateLimit)


#### Responses

| HTTP Code | Description | Schema     |
|:----------|:------------|:-----------|
| **200**   | OK          | `application/json` |


#### Consumes

* `application/json`


#### Example HTTP request

##### Request path

```
/api/v3/control-plane/rate-limits
```


##### Request body

```json
{
  "name": "my-rate-limit", 
  "limitRequestPerSecond": 100,
  "priority": "PROJECT"
}
```

#### Example HTTP response

##### Response 200


### Delete rate limit configuration

```
DELETE /api/v3/control-plane/rate-limits
```

Deletes rate limit configuration with the specified name and priority. 

#### Body parameter


*Name* : RateLimit  
*Flags* : required  
*Type* : [RateLimit](#RateLimit)


#### Responses

| HTTP Code | Description | Schema     |
|:----------|:------------|:-----------|
| **200**   | OK          | `application/json` |


#### Consumes

* `application/json`


#### Example HTTP request

##### Request path

```
/api/v3/control-plane/rate-limits
```


##### Request body

```json
{
  "name": "my-rate-limit", 
  "priority": "PROJECT"
}
```

#### Example HTTP response

##### Response 200


## Routes management - Version 3

Route Controller V 3

### Register virtual service with routes

```
POST /api/v3/control-plane/routes
```

#### Description

Create virtual service with routes for specified node-groups


#### Parameters

#### Body parameter

Information about virtual service with routes

*Name* : virtualServiceRegistrationRequest  
*Flags* : required  
*Type* : < [VirtualServiceRegistrationRequest](#virtualserviceregistrationrequest) > array

#### Responses

| HTTP Code | Description                 | Schema |
|:----------|:----------------------------|:-------|
| **201**   | Virtual service is created  |        |
| **400**   | Bad request                 |        |
| **500**   | Internal server error       |        |


#### Consumes

* `application/json`


#### Produces

* `application/json`

#### Example HTTP request

##### Request path

```
/api/v3/control-plane/routes
```

##### Request
```json
{
  "namespace" : "test-namespace",
  "gateways": ["gateway-1","gateway-2"],
  "listenerPort": 8443,
  "tlsSupported": true,
  "virtualServices": [
    {
      "name": "virtual-service-name",
      "hosts": [],
      "addHeaders": [
       {
         "name": "header1",
         "value": "value1"
       }
      ],
      "removeHeaders": ["Authorization"],
      "routeConfiguration": {
        "version": "v1",
        "routes": [
          {
            "destination": {
              "cluster": "test-cluster",
              "endpoint": "http://test-endpoint:8080",
              "tlsEndpoint": "https://test-endpoint:8443"
            },
            "rules": [
              {
                "match": {
                  "prefix": "/api/v1/test",
                  "headers": [
                    {
                      "name": "header1",
                      "exactMatch": "headerName1"
                    }
                  ]
                },
                "prefixRewrite": "/api/v1/",
                "addHeaders": [
                  {
                    "name": "header1",
                    "value": "value1"
                  }
                ],
                "removeHeaders": ["Authorization"],
                "timeout": 120000,
                "idleTimeout": 12000
              } 
            ]
          }
        ]
      } 
    }
  ]
}
```


### Create virtual service

```
POST /api/v3/control-plane/routes/{nodeGroup}/{virtualServiceName}
```

#### Description

Create virtual service with routes for specified node-group


#### Parameters

| Type      | Name                                      | Description               | Schema          |
|:----------|:------------------------------------------|:--------------------------|:----------------|
| **Path**  | **nodeGroup**           <br>*required*    | name of node group        | string          |
| **Path**  | **virtualServiceName**  <br>*required*    | name of virtual service   | string          |

#### Body parameter

Information about virtual service with routes

*Name* : virtualServiceRequest  
*Flags* : required  
*Type* : [VirtualService](#virtualservice)

#### Responses

| HTTP Code | Description             | Schema |
|:----------|:------------------------|:-------|
| **200**   | Virtual service updated |        |
| **400**   | Bad request             |        |
| **500**   | Internal server error   |        |


#### Consumes

* `application/json`


#### Produces

* `application/json`

#### Example HTTP request

##### Request path

```
/api/v3/control-plane/routes/internal-gateway-service/test-virtual-service
```

##### Request

```json
{
  "name": "virtual-service-name",
  "hosts": [],
  "addHeaders": [
    {
      "name": "header1",
      "value": "value1"
    }
  ],
  "removeHeaders": ["Authorization"],
  "routeConfiguration": {
    "version": "v1",
    "routes": [
      {
        "destination": {
          "cluster": "test-cluster",
          "endpoint": "https://test-endpoint:8443"
        },
        "rules": [
          {
            "match": {
              "prefix": "/api/v1/test",
              "headers": [
                {
                  "name": "header1",
                  "exactMatch": "headerName1"
                }
              ]
            },
            "prefixRewrite": "/api/v1/",
            "addHeaders": [
              {
                "name": "header1",
                "value": "value1"
              }
            ],
            "removeHeaders": ["Authorization"],
            "timeout": 120000,
            "idleTimeout": 12000
           } 
        ]
      }
    ]
  }
}
```


### Update virtual service

```
PUT /api/v3/control-plane/routes/{nodeGroup}/{virtualServiceName}
```

#### Description

Create or update virtual service with routes for specified node-group


#### Parameters

| Type      | Name                                      | Description               | Schema          |
|:----------|:------------------------------------------|:--------------------------|:----------------|
| **Path**  | **nodeGroup**           <br>*required*    | name of node group        | string          |
| **Path**  | **virtualServiceName**  <br>*required*    | name of virtual service   | string          |

#### Body parameter

Information about virtual service with routes

*Name* : virtualServiceUpdateRequest  
*Flags* : required  
*Type* : [VirtualService](#virtualservice)

#### Responses

| HTTP Code | Description             | Schema |
|:----------|:------------------------|:-------|
| **200**   | Virtual service updated |        |
| **400**   | Bad request             |        |
| **500**   | Internal server error   |        |


#### Consumes

* `application/json`


#### Produces

* `application/json`

#### Example HTTP request

##### Request path

```
/api/v3/control-plane/routes/internal-gateway-service/test-virtual-service
```

##### Request

```json
{
  "name": "virtual-service-name",
  "hosts": [],
  "addHeaders": [
    {
      "name": "header1",
      "value": "value1"
    }
  ],
  "removeHeaders": ["Authorization"],
  "routeConfiguration": {
    "version": "v1",
    "routes": [
      {
        "destination": {
          "cluster": "test-cluster",
          "endpoint": "https://test-endpoint:8443"
        },
        "rules": [
          {
            "match": {
              "prefix": "/api/v1/test",
              "headers": [
                {
                  "name": "header1",
                  "exactMatch": "headerName1"
                }
              ]
            },
            "prefixRewrite": "/api/v1/",
            "addHeaders": [
              {
                "name": "header1",
                "value": "value1"
              }
            ],
            "removeHeaders": ["Authorization"],
            "timeout": 120000,
            "idleTimeout": 12000
           } 
        ]
      }
    ]
  }
}
```

### Delete virtual service

```
DELETE /api/v3/control-plane/routes/{nodeGroup}/{virtualServiceName}
```

#### Description

Delete virtual service with routes for specified node-group


#### Parameters

| Type      | Name                                      | Description               | Schema          |
|:----------|:------------------------------------------|:--------------------------|:----------------|
| **Path**  | **nodeGroup**           <br>*required*    | name of node group        | string          |
| **Path**  | **virtualServiceName**  <br>*required*    | name of virtual service   | string          |

#### Responses

| HTTP Code | Description             | Schema |
|:----------|:------------------------|:-------|
| **200**   | Virtual service deleted |        |
| **400**   | Bad request             |        |
| **500**   | Internal server error   |        |


#### Consumes

* `application/json`


#### Produces

* `application/json`

#### Example HTTP request

##### Request path

```
/api/v3/control-plane/routes/internal-gateway-service/test-virtual-service
```

### Get virtual service

```
GET /api/v3/control-plane/routes/{nodeGroup}/{virtualServiceName}
```

#### Description

Get virtual service for specified node-group


#### Parameters

| Type      | Name                                      | Description               | Schema          |
|:----------|:------------------------------------------|:--------------------------|:----------------|
| **Path**  | **nodeGroup**           <br>*required*    | name of node group        | string          |
| **Path**  | **virtualServiceName**  <br>*required*    | name of virtual service   | string          |

#### Responses

| HTTP Code | Description | Schema                                    |
|:----------|:------------|:------------------------------------------|
| **200**   | OK          |  |

#### Responses

| HTTP Code | Description             | Schema                                            |
|:----------|:------------------------|:--------------------------------------------------|
| **200**   | Ok                      | [VirtualServiceResponse](#virtualserviceresponse) |
| **400**   | Bad request             |                                                   |
| **500**   | Internal server error   |                                                   |


#### Consumes

Empty

#### Produces

* `application/json`

#### Example HTTP request

##### Request path

```
/api/v3/control-plane/routes/internal-gateway-service/test-virtual-service
```

#### Example HTTP response

##### Response 200

```json
{ 
  "virtualHost": 
    {
      "name": "virtual-service-name",
      "version": 1,
      "domains": [],
      "addHeaders": [
        {
          "name": "header1",
          "value": "value1"
        }
     ],
    "removeHeaders": ["Authorization"],
    "routeConfiguration": {
    "version": "v1",
    "routes": [
      {
        "id": 896,
        "uuid": "529d7fa7-7c2c-493d-b5ff-f355357e4c54",
        "routeKey": "||\/api\/v2\/user-management\/tenants\/.*\/disable||v1",
        "matcher": {
          "prefix": null,
          "regExp": "/api/v2/user-management/tenants/(.*)/disable",
          "headers": [],
          "addHeaders": [],
          "removeHeaders": null
        },
        "action": {
          "clusterName": "identity-provider||identity-provider||8080",
          "hostRewrite": "identity-provider:8080",
          "hostAutoRewrite": null,
          "prefixRewrite": null,
          "regexpRewrite": "/auth/api/v2/user-management/tenants/\\1/disable",
          "pathRewrite": null
        },
        "directResponseAction": null,
        "version": 28,
        "timeout": null,
        "deploymentVersion": {
          "version": "v1",
          "stage": "ACTIVE",
          "createdWhen": "2020-10-12T09:12:28.389783Z",
          "updatedWhen": "2020-10-12T09:12:28.389783Z"
        },
        "initialDeploymentVersion": "v1",
        "autoGenerated": false,
        "hashPolicy": []
      }
    ]
    }
  },
  "clusters":
    {
      "id": 26,
      "name": "micro-service||micro-service||8080",
      "lbPolicy": "LEAST_REQUEST",
      "type": "STRICT_DNS",
      "version": 1,
      "enableH2": false,
      "nodeGroups": [
        {
          "name": "internal-gateway-service"
        }
      ],
      "endpoints": [
        {
          "id": 33,
          "address": "micro-service-v1",
          "port": 8080,
          "deploymentVersion": 
            {
              "version": "v1",
              "stage": "ACTIVE",
              "createdWhen": "2020-10-12T09:12:28.389783Z",
              "updatedWhen": "2020-10-12T09:12:28.389783Z"
            },
          "hashPolicy": []
        }
      ]
    }
}
```

### Delete virtual service domains

```
DELETE /api/v3/control-plane/domains
```

#### Description

Delete domains for virtual service for specified node-group

#### Parameters

#### Responses

| HTTP Code | Description             | Schema |
|:----------|:------------------------|:-------|
| **200**   | Ok                      |        |
| **400**   | Bad request             |        |
| **500**   | Internal server error   |        |

#### Body parameter

*Name*:  hostDeleteRequestV3  
*Flags*: required  
*Type*: < [DomainDeleteRequestV3](#DomainDeleteRequestV3) > array

#### Consumes

* `application/json`

#### Produces

* `application/json`

#### Example HTTP request

##### Request path

```
/api/v3/control-plane/domains
```

##### Request

```json
[
  {
    "virtualService": "private-gateway-service",
    "gateway": "private-gateway-service",
    "domains": [
      "test-domain.local:8080",
      "test-domain.local.svc:8080"
    ]
  }
]
```

### Delete endpoints

```http request
DELETE /api/v3/control-plane/endpoints
```

#### Body Parameters

deleteRequests

*Name* : endpointDeleteRequest  
*Flags* : required  
*Type* : < [EndpointDeleteRequest](#EndpointDeleteRequest) > array

#### Responses

| HTTP Code | Description | Scheme                          |
|:----------|:------------|:--------------------------------|
| **200**   | OK          | < [Endpoint](#Endpoint) > array |
| **400**   | Bad request |                                 |

#### Example request

Request path

```http request
DELETE /api/v3/control-plane/endpoints
```

Request body

```json
[
  {
    "endpoints": [
      {
        "address": "test-endpoint",
        "port": 8080
      }
    ],
    "version": "v1"
  }
]
```

#### Example response

Response 200

```json
[
  {
    "Id": 11,
    "Address": "test-endpoint",
    "Port": 8080,
    "ClusterId": 11,
    "Cluster": null,
    "DeploymentVersion": "v1",
    "InitialDeploymentVersion": "v1",
    "DeploymentVersionVal": {
      "version": "v1",
      "stage": "ACTIVE",
      "createdWhen": "2021-12-20T11:28:16.906402Z",
      "updatedWhen": "2021-12-20T11:28:16.906402Z"
    },
    "HashPolicies": null,
    "Hostname": "",
    "OrderId": 0
  }
]
```

### Configure tcp keepalive for cluster upstream connections

```
POST /api/v3/clusters/tcp-keepalive
```

#### Description

The endpoint allows to register tcp keepalive for cluster upstream connections. 

This API is available only on control-plane directly and not exposed to any gateways.

Requires one of the following roles: ROLE_M2M, ROLE_devops-client, ROLE_CLOUD-ADMIN, ROLE_key-manager

#### Request Body

[ClusterTcpKeepalive](#clustertcpkeepalive)

#### Responses

| HTTP Code | Description | Schema |
|:----------|:------------|:-------|
| **200**   | OK          | none   |
| **400**   | Bad Request - in case cluster does not exist.  | none   |

#### Produces

* `\*/*`
* `application/json`


## Universal configure endpoint

### Apply configuration

```
POST /api/v3/apply-config
```

#### Description

The endpoint provides opportunity to apply different kinds of configuration with k8s like format.
Every example of configuration from docs/config directory is supported. Yaml and Json are both supported.
Multiple yaml format with "---" delimiter is also supported.
As a result you get list of entities which were applied and status of operation.
If something went wrong you would get 400 or 500 code from CP or 200 in case of successful processing of all entities,
in details you would be able to see real code of each entity, it can be 500 or 200.

#### Example HTTP request
##### Request path
```
/api/v3/apply-config
```
##### Request body
```yaml
apiVersion: nc.core.mesh/v3
kind: RouteConfiguration
metadata:
  name: tenant-manager-public-routes
  namespace: cloud-core
spec:
  gateways: ["public-gateway-service"]
  tlsSupported: true
  virtualServices:
    - name: public-gateway-service
      hosts: ["*"]
      routeConfiguration:
        version: v1
        routes:
          - destination:
              cluster: tenant-manager
              endpoint: http://tenant-manager-v1:8080
              tlsEndpoint: https://tenant-manager-v1:8443
            rules:
              - match:
                  prefix: /api/v4/tenant-manager/public-api
                prefixRewrite: /api/v4/api
---
apiVersion: nc.core.mesh/v3
kind: RouteConfiguration
metadata:
  name: tenant-manager-private-routes
  namespace: cloud-core
spec:
  gateways: ["private-gateway-service"]
  tlsSupported: true
  virtualServices:
    - name: private-gateway-service
      hosts: ["*"]
      routeConfiguration:
        version: v1
        routes:
          - destination:
              cluster: tenant-manager
              endpoint: http://tenant-manager-v1:8080
              tlsEndpoint: https://tenant-manager-v1:8443
            rules:
              - match:
                  prefix: /api/v4/tenant-manager/private-api
                prefixRewrite: /api/v4/api
---
apiVersion: nc.core.mesh/v3
kind: RouteConfiguration
metadata:
  name: trace-service-mesh-routes
  namespace: cloud-core
spec:
  gateways: [ "trace-services-gateway" ]
  listenerPort: 1234
  tlsSupported: true
  virtualServices:
    - name: "trace-service"
      hosts: ["trace-service"]
      routeConfiguration:
        version: v1
        routes:
          - destination:
              cluster: "trace-service:1234"
              endpoint: http://trace-service-v1:1234
            rules:
              - match:
                  prefix: /trace-service/trace/1234
                allowed: true
                prefixRewrite: /trace/1234
```

#### Example HTTP response
##### Response 200

```json
[
  {
    "request": {
      "apiVersion": "nc.core.mesh/v3",
      "kind": "RouteConfiguration",
      "nodeGroup": "",
      "metadata": {
        "name": "tenant-manager-public-routes",
        "namespace": "cloud-core",
        "nodeGroup": ""
      },
      "spec": {
        "gateways": [
          "public-gateway-service"
        ],
        "listenerPort": 1234,
        "tlsSupported": true,
        "virtualServices": [
          {
            "hosts": [
              "*"
            ],
            "name": "public-gateway-service",
            "routeConfiguration": {
              "routes": [
                {
                  "destination": {
                    "cluster": "tenant-manager",
                    "endpoint": "http://tenant-manager-v1:8080",
                    "tlsEndpoint": "https://tenant-manager-v1:8443"
                  },
                  "rules": [
                    {
                      "match": {
                        "prefix": "/api/v4/tenant-manager/public-api"
                      },
                      "prefixRewrite": "/api/v4/api"
                    }
                  ]
                }
              ],
              "version": "v1"
            }
          }
        ]
      }
    },
    "response": {
      "code": 200,
      "error": "",
      "data": null
    }
  },
  {
    "request": {
      "apiVersion": "nc.core.mesh/v3",
      "kind": "RouteConfiguration",
      "nodeGroup": "",
      "metadata": {
        "name": "tenant-manager-private-routes",
        "namespace": "cloud-core",
        "nodeGroup": ""
      },
      "spec": {
        "gateways": [
          "private-gateway-service"
        ],
        "tlsSupported": true,
        "virtualServices": [
          {
            "hosts": [
              "*"
            ],
            "name": "private-gateway-service",
            "routeConfiguration": {
              "routes": [
                {
                  "destination": {
                    "cluster": "tenant-manager",
                    "endpoint": "http://tenant-manager-v1:8080",
                    "tlsEndpoint": "https://tenant-manager-v1:8443"
                  },
                  "rules": [
                    {
                      "match": {
                        "prefix": "/api/v4/tenant-manager/private-api"
                      },
                      "prefixRewrite": "/api/v4/api"
                    }
                  ]
                }
              ],
              "version": "v1"
            }
          }
        ]
      }
    },
    "response": {
      "code": 200,
      "error": "",
      "data": null
    }
  },
  {
    "request": {
      "apiVersion": "nc.core.mesh/v3",
      "kind": "RouteConfiguration",
      "nodeGroup": "",
      "metadata": {
        "name": "trace-service-mesh-routes",
        "namespace": "cloud-core",
        "nodeGroup": ""
      },
      "spec": {
        "gateways": [
          "trace-services-gateway"
        ],
        "virtualServices": [
          {
            "hosts": [
              "trace-service"
            ],
            "name": "trace-service",
            "routeConfiguration": {
              "routes": [
                {
                  "destination": {
                    "cluster": "trace-service:1234",
                    "endpoint": "http://trace-service-v1:1234"
                  },
                  "rules": [
                    {
                      "match": {
                        "prefix": "/trace-service/trace/1234"
                      },
                      "prefixRewrite": "/trace/1234"
                    }
                  ]
                }
              ],
              "version": "v1"
            }
          }
        ]
      }
    },
    "response": {
      "code": 200,
      "error": "",
      "data": null
    }
  }
]
```

#### Supported configurations
* Old format
    ```yaml
    nodeGroup: private-gateway-service
    spec:
    - namespace: "${ENV_NAMESPACE}"
      cluster: "${ENV_SERVICE_NAME}"
      endpoint: http://${ENV_DEPLOYMENT_RESOURCE_NAME}:8080
      routes:
        - prefix: "/trace-service/health"
          prefixRewrite: "/health"
      version: "${ENV_DEPLOYMENT_VERSION}"
      allowed: true
    ```
* nc.core.mesh/v3 RouteConfiguration
    ```yaml
    apiVersion: nc.core.mesh/v3
    kind: RouteConfiguration
    metadata:
      name: tenant-manager-routes
      namespace: cloud-core
    spec:
      gateways: ["ingress-gateway"]
      virtualServices:
        - name: public-gateway-service
          routeConfiguration:
            version: v1
            routes:
            - destination:
                cluster: tenant-manager
                endpoint: http://tenant-manager-v1:8080
              rules:
                - match:
                    prefix: /api/v4/tenant-manager/tenants
                  prefixRewrite: /api/v4/tenants
    ```
* nc.core.mesh/v3 RouteConfiguration for facade/composite gateways
    ```yaml
    apiVersion: nc.core.mesh/v3
    kind: RouteConfiguration
    metadata:
      name: tenant-manager-routes
      namespace: cloud-core
    spec:
      gateways: ["facade-gateway"]
      listenerPort: 8080
      tlsSupported: true
      virtualServices:
        - name: facade-gateway-service
          routeConfiguration:
            version: v1
            routes:
            - destination:
                cluster: test-service
                endpoint: http://test-service-v1:8080
                tlsEndpoint: https://test-service-v1:8443
                circuitBreaker:
                threshold:
                  maxConnections: 2 // USE IT ONLY WITH "facadeGatewayConcurrency": 1
              rules:
                - match:
                    prefix: /api/v1/test-service/test
                  prefixRewrite: /api/v1/test
    ```
* nc.core.mesh/v3 VirtualService
    ```yaml
    apiVersion: nc.core.mesh/v3
    kind: VirtualService
    metadata:
      name: virtual-service-name
      gateway: public-gateway-service
    spec:
      routeConfiguration:
        version: v1
        routes:
          - destination:
              cluster: tenant-manager
              endpoint: http://tenant-manager-v1:8080
            rules:
              - match:
                  prefix: /api/v4/tenant-manager/tenants
                prefixRewrite: /api/v4/tenants  
    ```
* nc.core.mesh/v3 RoutesDrop
    ```yaml
    apiVersion: nc.core.mesh/v3
    kind: RoutesDrop
    metadata:
      name: delete-old-trace-drop
      namespace: cloud-core
    spec:
    - gateways: ["ingress-gateway"]
      virtualService: trace-service
      routes:
        - prefix: /trace
      version: v2
    ```
* LoadBalance
    ```yaml
    APIVersion: nc.core.mesh/v3
    kind: LoadBalance
    metadata:
      name: quot-eng-lb
      namespace: cloud-core
    spec:
      cluster: "quotation-engine"
      version: "v1"
      endpoint: http://quotation-engine-v1:8080
      policies:
      - header:
          headerName: "BID"
      - cookie:
          name: "JSESSION"
          ttl: 5
    ```
## Composite Platform functionality

### Get Composite Platform structure

```
GET /api/v3/composite-platform/namespaces
```

#### Description

The endpoint provides ability to get information about Composite Platform namespaces structure: 
which namespace is a baseline, and what satellites namespaces are there. 

This API is available on internal-gateways and control-plane instances in every namespace of the Composite Platform environment. 

Requires one of the following roles: ROLE_M2M, ROLE_devops-client, ROLE_CLOUD-ADMIN, ROLE_key-manager

#### Example HTTP request
##### Request path
```
/api/v3/composite-platform/namespaces
```

#### Responses

| HTTP Code | Description | Schema                            |
|:----------|:------------|:----------------------------------|
| **200**   | OK          | [CompositePlatformStructure](#compositeplatformstructure) |


#### Produces

* `\*/*`
* `application/json`

#### Example HTTP response
##### Response 200

```json
{
  "baseline": "core-base", 
  "satellites": ["satellite-1", "satellite-2"]
}
```

### Add satellite to Composite Platform structure

```
POST /api/v3/composite-platform/namespaces/{namespace}
```

#### Description

Provides ability to add new satellite namespace to the Composite Platform structure. 

This API is available on internal-gateways and control-plane instances in every namespace of the Composite Platform environment.

Requires one of the following roles: ROLE_M2M, ROLE_devops-client, ROLE_CLOUD-ADMIN, ROLE_key-manager

#### Example HTTP request
##### Request path
```
/api/v3/composite-platform/namespaces/satellite-1
```

#### Responses

| HTTP Code | Description | Schema                            |
|:----------|:------------|:----------------------------------|
| **200**   | OK          |  |

#### Example HTTP response
##### Response 200

### Remove satellite from Composite Platform structure

```
DELETE /api/v3/composite-platform/namespaces/{namespace}
```

#### Description

Provides ability to remove satellite namespace from the Composite Platform structure stored in control-plane. 

This API is available on internal-gateways and control-plane instances in every namespace of the Composite Platform environment.

Requires one of the following roles: ROLE_M2M, ROLE_devops-client, ROLE_CLOUD-ADMIN, ROLE_key-manager

#### Example HTTP request
##### Request path
```
/api/v3/composite-platform/namespaces/satellite-1
```

#### Responses

| HTTP Code | Description | Schema                            |
|:----------|:------------|:----------------------------------|
| **200**   | OK          |  |

#### Example HTTP response
##### Response 200

## Declarative gateways functionality

### Get all gateway declarations

```
GET /api/v3/control-plane/gateways/specs
```

#### Description

The endpoint responds with all gateway declarations registered in control-plane. 

This API is available on private and internal gateway and control-plane directly (remove `/control-plane` from path when calling control-plane directly).

Requires one of the following roles: ROLE_M2M, ROLE_devops-client, ROLE_CLOUD-ADMIN, ROLE_key-manager

#### Responses

| HTTP Code | Description | Schema                                              |
|:----------|:------------|:----------------------------------------------------|
| **200**   | OK          | < [GatewayDeclaration](#GatewayDeclaration) > array |

#### Produces

* `\*/*`
* `application/json`

### Register gateway declaration

```
POST /api/v3/control-plane/gateways/specs
```

#### Description

The endpoint allows to register new gateway declaration or modify an existing one. 

This API is available on private and internal gateway and control-plane directly (remove `/control-plane` from path when calling control-plane directly).

Requires one of the following roles: ROLE_M2M, ROLE_devops-client, ROLE_CLOUD-ADMIN, ROLE_key-manager

#### Request Body

[GatewayDeclaration](#GatewayDeclaration)

#### Responses

| HTTP Code | Description                                                                                                                                        | Schema |
|:----------|:---------------------------------------------------------------------------------------------------------------------------------------------------|:-------|
| **200**   | OK                                                                                                                                                 | none   |
| **400**   | Bad Request - in case there is some validation error, or existing gateway specification cannot be changed due to conflicting routes configuration. | object |

#### Produces

* `\*/*`
* `application/json`

### Drop gateway declaration

```
DELETE /api/v3/control-plane/gateways/specs
```

#### Description

The endpoint allows to delete gateway declaration. 

This API is available on private and internal gateway and control-plane directly (remove `/control-plane` from path when calling control-plane directly).

Requires one of the following roles: ROLE_M2M, ROLE_devops-client, ROLE_CLOUD-ADMIN, ROLE_key-manager

#### Request Body

[GatewayDeclaration](#GatewayDeclaration)

#### Responses

| HTTP Code | Description                                                                                                                                                 | Schema |
|:----------|:------------------------------------------------------------------------------------------------------------------------------------------------------------|:-------|
| **200**   | OK                                                                                                                                                          | none   |
| **400**   | Bad Request - in case there is some validation error, or existing gateway specification cannot be deleted as it is still in use by some routes or clusters. | object |

#### Produces

* `\*/*`
* `application/json`


## Gateway HTTP Filters

### Get all HTTP filters

```
GET /api/v3/control-plane/http-filters/{gateway}
```

#### Description

The endpoint responds with all HTTP filters configured for the gateway specified as request path variable.

This API is available on private and internal gateway and control-plane directly (remove `/control-plane` from path when calling control-plane directly).

Requires one of the following roles: ROLE_M2M, ROLE_devops-client, ROLE_CLOUD-ADMIN, ROLE_key-manager

#### Responses

| HTTP Code | Description | Schema                                                              |
|:----------|:------------|:--------------------------------------------------------------------|
| **200**   | OK          | < [HTTPFiltersConfigRequestV3](#HTTPFiltersConfigRequestV3) > array |

#### Produces

* `\*/*`
* `application/json`

### Register HTTP filters

```
POST /api/v3/control-plane/http-filters
```

#### Description

The endpoint allows to register new HTTP filters or modify existing ones. 

This API is available on private and internal gateway and control-plane directly (remove `/control-plane` from path when calling control-plane directly).

Requires one of the following roles: ROLE_M2M, ROLE_devops-client, ROLE_CLOUD-ADMIN, ROLE_key-manager

#### Request Body

[HTTPFiltersConfigRequestV3](#HTTPFiltersConfigRequestV3)

#### Responses

| HTTP Code | Description | Schema |
|:----------|:------------|:-------|
| **200**   | OK          | none   |

#### Produces

* `\*/*`
* `application/json`

### Drop HTTP filters

```
DELETE /api/v3/control-plane/http-filters
```

#### Description

The endpoint allows to delete HTTP filters. 

This API is available on private and internal gateway and control-plane directly (remove `/control-plane` from path when calling control-plane directly).

Requires one of the following roles: ROLE_M2M, ROLE_devops-client, ROLE_CLOUD-ADMIN, ROLE_key-manager

#### Request Body

[HttpFiltersDropConfigRequestV3](#HttpFiltersDropConfigRequestV3)

#### Responses

| HTTP Code | Description | Schema |
|:----------|:------------|:-------|
| **200**   | OK          | none   |

#### Produces

* `\*/*`
* `application/json`

### GetNamespace

```
GET /api/v3/control-plane/namespace
```

#### Description

The endpoint allows to get actual namespace

This API is available on private gateway.

Does not require any role to use this API

#### Responses

| HTTP Code | Description | Schema |
|:----------|:------------|:-------|
| **200**   | OK          | string |

#### Produces

* `\*/*`
* `text/plain`

## Debug API

### GetConfigValidationReport

```
GET /api/v3/control-plane/debug/config-validation
```

# Definitions

## DeferredResult

| Name                             | Description              | Schema  |
|:---------------------------------|:-------------------------|:--------|
| **result**  <br>*optional*       | **Example** : `"object"` | object  |
| **setOrExpired**  <br>*optional* | **Example** : `true`     | boolean |

## DeferredResult«ResponseEntity«object»»

| Name                             | Description              | Schema  |
|:---------------------------------|:-------------------------|:--------|
| **result**  <br>*optional*       | **Example** : `"object"` | object  |
| **setOrExpired**  <br>*optional* | **Example** : `true`     | boolean |


## DeploymentVersion

| Name                            | Description              | Schema                                     |
|:--------------------------------|:-------------------------|:-------------------------------------------|
| **createdWhen**  <br>*optional* | **Example** : `"string"` | string (date-time)                         |
| **stage**  <br>*optional*       | **Example** : `"string"` | enum (LEGACY, ACTIVE, CANDIDATE, ARCHIVED) |
| **updatedWhen**  <br>*optional* | **Example** : `"string"` | string (date-time)                         |
| **version**  <br>*optional*     | **Example** : `"string"` | string                                     |

## MicroserviceVersion

| Name                            | Description              | Schema                                     |
|:--------------------------------|:-------------------------|:-------------------------------------------|
| **version**  | Value for `X-Version` header to be set by microservice on outgoing requests. If this field is empty string, no `X-Version` header should be added by microservice at all. | string                                     |

## DirectResponseAction

| Name                       | Description       | Schema          |
|:---------------------------|:------------------|:----------------|
| **status**  <br>*optional* | **Example** : `0` | integer (int32) |



## HeaderMatcher

| Name                              | Description              | Schema          |
|:----------------------------------|:-------------------------|:----------------|
| **id**  <br>*optional*            | **Example** : `0`        | integer (int64) |
| **name**  <br>*optional*          | **Example** : `"string"` | string          |
| **version**  <br>*optional*       | **Example** : `0`        | integer (int64) |
| **exactMatch**  <br>*optional*    | **Example** : `"string"` | string |
| **safeRegexMatch** <br>*optional* | **Example** : `"string"` | string |
| **rangeMatch**  <br>*optional*    | **Example** : `"[rangematch](#rangematch)"` | < [RangeMatch](#rangematch) > |
| **presentMatch**  <br>*optional*  | **Example** : `"string"` | string |
| **prefixMatch**  <br>*optional*   | **Example** : `"string"` | string |
| **suffixMatch**  <br>*optional*   | **Example** : `"string"` | string |
| **invertMatch**  <br>*optional*   | **Example** : `"string"` | string |


## ResponseEntity

| Name                                | Description              | Schema                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
|:------------------------------------|:-------------------------|:----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **body**  <br>*optional*            | **Example** : `"object"` | object                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| **statusCode**  <br>*optional*      | **Example** : `"string"` | enum (100 CONTINUE, 101 SWITCHING_PROTOCOLS, 102 PROCESSING, 103 CHECKPOINT, 200 OK, 201 CREATED, 202 ACCEPTED, 203 NON_AUTHORITATIVE_INFORMATION, 204 NO_CONTENT, 205 RESET_CONTENT, 206 PARTIAL_CONTENT, 207 MULTI_STATUS, 208 ALREADY_REPORTED, 226 IM_USED, 300 MULTIPLE_CHOICES, 301 MOVED_PERMANENTLY, 302 FOUND, 302 MOVED_TEMPORARILY, 303 SEE_OTHER, 304 NOT_MODIFIED, 305 USE_PROXY, 307 TEMPORARY_REDIRECT, 308 PERMANENT_REDIRECT, 400 BAD_REQUEST, 401 UNAUTHORIZED, 402 PAYMENT_REQUIRED, 403 FORBIDDEN, 404 NOT_FOUND, 405 METHOD_NOT_ALLOWED, 406 NOT_ACCEPTABLE, 407 PROXY_AUTHENTICATION_REQUIRED, 408 REQUEST_TIMEOUT, 409 CONFLICT, 410 GONE, 411 LENGTH_REQUIRED, 412 PRECONDITION_FAILED, 413 PAYLOAD_TOO_LARGE, 413 REQUEST_ENTITY_TOO_LARGE, 414 URI_TOO_LONG, 414 REQUEST_URI_TOO_LONG, 415 UNSUPPORTED_MEDIA_TYPE, 416 REQUESTED_RANGE_NOT_SATISFIABLE, 417 EXPECTATION_FAILED, 418 I_AM_A_TEAPOT, 419 INSUFFICIENT_SPACE_ON_RESOURCE, 420 METHOD_FAILURE, 421 DESTINATION_LOCKED, 422 UNPROCESSABLE_ENTITY, 423 LOCKED, 424 FAILED_DEPENDENCY, 426 UPGRADE_REQUIRED, 428 PRECONDITION_REQUIRED, 429 TOO_MANY_REQUESTS, 431 REQUEST_HEADER_FIELDS_TOO_LARGE, 451 UNAVAILABLE_FOR_LEGAL_REASONS, 500 INTERNAL_SERVER_ERROR, 501 NOT_IMPLEMENTED, 502 BAD_GATEWAY, 503 SERVICE_UNAVAILABLE, 504 GATEWAY_TIMEOUT, 505 HTTP_VERSION_NOT_SUPPORTED, 506 VARIANT_ALSO_NEGOTIATES, 507 INSUFFICIENT_STORAGE, 508 LOOP_DETECTED, 509 BANDWIDTH_LIMIT_EXCEEDED, 510 NOT_EXTENDED, 511 NETWORK_AUTHENTICATION_REQUIRED) |
| **statusCodeValue**  <br>*optional* | **Example** : `0`        | integer (int32)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |


## Route

| Name                                         | Description                                                     | Schema                                        |
|:---------------------------------------------|:----------------------------------------------------------------|:----------------------------------------------|
| **action**  <br>*optional*                   | **Example** : `"[routeaction](#routeaction)"`                   | [RouteAction](#routeaction)                   |
| **autoGenerated**  <br>*optional*            | **Example** : `true`                                            | boolean                                       |
| **deploymentVersion**  <br>*optional*        | **Example** : `"[deploymentversion](#deploymentversion)"`       | [DeploymentVersion](#deploymentversion)       |
| **directResponseAction**  <br>*optional*     | **Example** : `"[directresponseaction](#directresponseaction)"` | [DirectResponseAction](#directresponseaction) |
| **id**  <br>*optional*                       | **Example** : `0`                                               | integer (int64)                               |
| **initialDeploymentVersion**  <br>*optional* | **Example** : `"string"`                                        | string                                        |
| **matcher**  <br>*optional*                  | **Example** : `"[routematcher](#routematcher)"`                 | [RouteMatcher](#routematcher)                 |
| **routeKey**  <br>*optional*                 | **Example** : `"string"`                                        | string                                        |
| **timeout**  <br>*optional*                  | **Example** : `0`                                               | integer (int64)                               |
| **timeoutSeconds**  <br>*optional*           | **Example** : `0`                                               | integer (int64)                               |
| **version**  <br>*optional*                  | **Example** : `0`                                               | integer (int64)                               |


## RouteAction

| Name                                | Description              | Schema  |
|:------------------------------------|:-------------------------|:--------|
| **clusterName**  <br>*optional*     | **Example** : `"string"` | string  |
| **hostAutoRewrite**  <br>*optional* | **Example** : `true`     | boolean |
| **hostRewrite**  <br>*optional*     | **Example** : `"string"` | string  |
| **pathRewrite**  <br>*optional*     | **Example** : `"string"` | string  |
| **prefixRewrite**  <br>*optional*   | **Example** : `"string"` | string  |


## RouteDeleteRequest

| Name                          | Description                                   | Schema                            |
|:------------------------------|:----------------------------------------------|:----------------------------------|
| **namespace**  <br>*optional* | **Example** : `"string"`                      | string                            |
| **routes**  <br>*optional*    | **Example** : `[ "[routeitem](#routeitem)" ]` | < [RouteItem](#routeitem) > array |
| **version**  <br>*optional*   | **Example** : `"string"`                      | string                            |


## RouteEntityRequest

| Name                                | Description                                     | Schema                              |
|:------------------------------------|:------------------------------------------------|:------------------------------------|
| **allowed**  <br>*optional*         | **Example** : `true`                            | boolean                             |
| **microserviceUrl**  <br>*optional* | **Example** : `"string"`                        | string                              |
| **routes**  <br>*optional*          | **Example** : `[ "[routeentry](#routeentry)" ]` | < [RouteEntry](#routeentry) > array |


## RouteEntry

| Name                          | Description              | Schema          |
|:------------------------------|:-------------------------|:----------------|
| **from**  <br>*optional*      | **Example** : `"string"` | string          |
| **namespace**  <br>*optional* | **Example** : `"string"` | string          |
| **timeout**  <br>*optional*   | **Example** : `0`        | integer (int64) |
| **to**  <br>*optional*        | **Example** : `"string"` | string          |


## RouteItem

| Name                       | Description              | Schema |
|:---------------------------|:-------------------------|:-------|
| **prefix**  <br>*optional* | **Example** : `"string"` | string |


## RouteMatcher

| Name                        | Description                                                 | Schema                                          |
|:----------------------------|:------------------------------------------------------------|:------------------------------------------------|
| **headers**  <br>*optional* | **Example** : `[ "[headermatcher](#headermatcher)" ]`       | < [HeaderMatcher](#headermatcher) > array       |
| **path**  <br>*optional*    | **Example** : `"string"`                                    | string                                          |
| **prefix**  <br>*optional*  | **Example** : `"string"`                                    | string                                          |
| **regExp**  <br>*optional*  | **Example** : `"string"`                                    | string                                          |
| **addHeaders**              | **Example** : `[ "[HeaderDefinition](#headerdefinition)" ]` | < [HeaderDefinition](#headerdefinition) > array |
| **removeHeaders**           | **Example** : `["header"]`                                  | < string > array                                |


## RouteRegistrationRequest

| Name                          | Description                                     | Schema                              |
|:------------------------------|:------------------------------------------------|:------------------------------------|
| **allowed**  <br>*optional*   | **Example** : `true`                            | boolean                             |
| **cluster**  <br>*optional*   | **Example** : `"string"`                        | string                              |
| **endpoint**  <br>*optional*  | **Example** : `"string"`                        | string                              |
| **namespace**  <br>*optional* | **Example** : `"string"`                        | string                              |
| **routes**  <br>*optional*    | **Example** : `[ "[routesitem](#routesitem)" ]` | < [RoutesItem](#routesitem) > array |
| **version**  <br>*optional*   | **Example** : `"string"`                        | string                              |


## RoutesItem

| Name                              | Description              | Schema |
|:----------------------------------|:-------------------------|:-------|
| **prefix**  <br>*optional*        | **Example** : `"string"` | string |
| **prefixRewrite**  <br>*optional* | **Example** : `"string"` | string |
| **headerMatchers** <br>*optional* | **Example** : `[ "[headermatcherdto](#headermatcherdto)" ]` | < [HeaderMatcherDto](#headermatcherdto) > array |

## HeaderMatcherDto

| Name                              | Description              | Schema |
|:----------------------------------|:-------------------------|:-------|
| **name**                          | **Example** : `"string"` | string |
| **exactMatch**  <br>*optional*    | **Example** : `"string"` | string |
| **safeRegexMatch** <br>*optional* | **Example** : `"string"` | string |
| **rangeMatch**  <br>*optional*    | **Example** : `"[rangematch](#rangematch)"` | < [RangeMatch](#rangematch) > |
| **presentMatch**  <br>*optional*  | **Example** : `"string"` | string |
| **prefixMatch**  <br>*optional*   | **Example** : `"string"` | string |
| **suffixMatch**  <br>*optional*   | **Example** : `"string"` | string |
| **invertMatch**  <br>*optional*   | **Example** : `"string"` | string |

## RangeMatch

| Name                              | Description              | Schema          |
|:----------------------------------|:-------------------------|:----------------|
| **start**                         | **Example** : `0`        | integer (int64) |
| **end**                           | **Example** : `10`       | integer (int64) |

## RoutingModeDetails

| Name                            | Description                  | Schema                                      |
|:--------------------------------|:-----------------------------|:--------------------------------------------|
| **routeKeys**  <br>*optional*   | **Example** : `[ "string" ]` | < string > array                            |
| **routingMode**  <br>*optional* | **Example** : `"string"`     | enum (SIMPLE, NAMESPACED, VERSIONED, MIXED) |


## LoadBalanceSpec

| Name                          | Description                                                                       | Schema                              |
|:------------------------------|:----------------------------------------------------------------------------------|:------------------------------------|
| **cluster**                   | **Example** : `"my_service_name"`                                                 | string                              |
| **version**    <br>*optional* | **Example** : `"v1"`                                                              | string                              |
| **endpoint**                  | **Example** : `"http://service_name:8080"`                                   | string                              |
| **namespace**  <br>*optional* | **Example** : `"my_namespace"`                                                    | string                              |
| **policies**                  | **Example** : `[{"header":{"headerName":"BID"}, "cookie":{"name":"JSESSIONID"}}]` | <[HashPolicy](#hashpolicy)> array |


## HashPolicy

| Name                          | Description                                     | Schema                              |
|:------------------------------|:------------------------------------------------|:------------------|
| **header**                    | **Example** : `{"header":{"headerName":"BID"}}` | [Header](#header) |
| **cookie**  <br>*optional*    | **Example** : `{"cookie":{"name":"JSESSIONID"}}`| [Cookie](#cookie) |


## Header

| Name                              | Description              | Schema |
|:----------------------------------|:-------------------------|:-------|
| **headerName**                    | **Example** : `"BID"`    | string |


## Cookie

| Name                              | Description                  | Schema          |
|:----------------------------------|:-----------------------------|:----------------|
| **name**  <br>*optional*          | **Example** : `"JSESSIONID"` | string          |
| **ttl**  <br>*optional*           | **Example** : `0`            | integer (int64) |
| **path**  <br>*optional*          | **Example** : `"/mypath"`    | string          |


## VirtualServiceRegistrationRequest
| Name                              | Description                                              | Schema                                      |
|:----------------------------------|:---------------------------------------------------------|:--------------------------------------------|
| **namespace**  <br>*optional*     | **Example** : `"namespace"`                              | string                                      |
| **gateways**                      | **Example** : `["gateway1"]`                             | < string > array                            |
| **virtualServices**               | **Example** : `[ "[VirtualService](#virtualservice)" ]`  | < [VirtualService](#virtualservice) > array |


## VirtualServiceResponse
| Name                 | Description                                    | Schema                        |
|:---------------------|:-----------------------------------------------|:------------------------------|
| **virtualHost**      | **Example** : `"[VirtualHost](#virtualhost)"`  | [VirtualHost](#virtualhost)   |
| **clusters**         | **Example** : `[ "[Cluster](#cluster)" ]`      | < [Cluster](#cluster) > array |


## Cluster
| Name              | Description                                    | Schema                            |
|:------------------|:-----------------------------------------------|:----------------------------------|
| **id**            | **Example** : `1`                              | int32                             |
| **name**          | **Example** : `cluster`                        | string                            |
| **LbPolicy**      | **Example** : `LEAST_REQUEST`                  | string                            |
| **DiscoveryType** | **Example** : `STRICT_DNS`                     | string                            |
| **Version**       | **Example** : `1`                              | int32                             |
| **EnableH2**      | **Example** : `false`                          | bool                              |
| **nodeGroups**    | **Example** : `[ "[NodeGroup](#nodegroup)" ]`  | < [NodeGroup](#nodegroup) > array |
| **endpoints**     | **Example** : `[ "[Endpoint](#endpoint)" ]`    | < [Endpoint](#endpoint) > array   |


## NodeGroup
| Name              | Description                            | Schema      |
|:------------------|:---------------------------------------|:------------|
| **name**          | **Example** : `public-gateway-service` | string      |


## Endpoint
| Name                   | Description                                                  | Schema                                    |
|:-----------------------|:-------------------------------------------------------------|:------------------------------------------|
| **id**                 | **Example** : `1`                                            | int32                                     |
| **address**            | **Example** : `http://some-address`                          | string                                    |
| **port**               | **Example** : `8080`                                         | int32                                     |
| **deploymentVersion**  | **Example** : `"[deploymentversion](#deploymentversion)"`    | [DeploymentVersion](#deploymentversion)   |
| **hashPolicy**         | **Example** : `[ "[HashPolicy](#hashpolicy)" ]`              | < [HashPolicy](#hashpolicy) > array       |


## VirtualHost
| Name              | Description                                                 | Schema                                            |
|:------------------|:------------------------------------------------------------|:--------------------------------------------------|
| **id**            | **Example** : `1`                                           | int32                                              |
| **name**          | **Example** : `virtual-host`                                | string                                            |
| **addHeaders**    | **Example** : `[ "[HeaderDefinition](#headerdefinition)" ]` | < [HeaderDefinition](#headerdefinition) > array   |
| **removeHeaders** | **Example** : `["header"]`                                  | < string > array                                  |
| **routes**        | **Example** : `"[Route](#route)"`                           | < [Route](#route) > array                         |
| **domains**       | **Example** : `["*"]`                                       | < string > array                                  |


## VirtualService
| Name                              | Description                                                  | Schema                                            |
|:----------------------------------|:-------------------------------------------------------------|:--------------------------------------------------|
| **name**                          | **Example** : `"vitrual-service"`                            | string                                            |
| **hosts**  <br>*optional*         | **Example** : `["host"]`                                     | < string > array                                  |
| **addHeaders**  <br>*optional*    | **Example** : `[ "[HeaderDefinition](#headerdefinition)" ]`  | < [HeaderDefinition](#headerdefinition) > array   |
| **removeHeaders**  <br>*optional* | **Example** : `["header"]`                                   | < string > array                                  |
| **rateLimit**  <br>*optional* | **Example** : `"my-rate-limit"`                                   | string |
| **routeConfiguration**            | **Example** : `"[RouteConfiguration](#routeconfiguration)"`  | [RouteConfiguration](#routeconfiguration)         |


## RouteConfiguration
| Name                         | Description                                | Schema                        |
|:-----------------------------|:-------------------------------------------|:------------------------------|
| **version**  <br>*optional*  | **Example** : `"v1"`                       | string                        |
| **routes**                   | **Example** : `[ "[RouteV3](#routev3)" ]`  | < [RouteV3](#routev3) > array |


## RouteV3
| Name              | Description                                    | Schema                       |
|:------------------|:-----------------------------------------------|:-----------------------------|
| **destination**   | **Example** : `"[Destination](#destination)"`  | [Destination](#destination)  |
| **rules**         | **Example** : `[ "[Rule](#rule)" ]`            | < [Rule](#rule) > array      |

## Rule
| Name                              | Description                                                 | Schema                                            |
|:----------------------------------|:------------------------------------------------------------|:--------------------------------------------------|
| **match**                         | **Example** : `"[RouteMatcher](#routematcher)"`             | [RouteMatcher](#routematcher)                     |
| **allowed**  <br>*optional*       | **Example** : `true`                                        | boolean                                           |
| **deny**  <br>*optional*          | if true then all routes matched by provided prefix will be forbidden <br>**Example** : `true`                                        | boolean                                           |
| **prefixRewrite**  <br>*optional* | **Example** : `"http://test-cluster:8080"`                  | string                                            |
| **hostRewrite**  <br>*optional*   | **Example** : `"my-custom-host"`                            | string                                            |
| **addHeaders**  <br>*optional*    | **Example** : `[ "[HeaderDefinition](#headerdefinition)" ]` | < [HeaderDefinition](#headerdefinition) > array   |
| **removeHeaders**  <br>*optional* | **Example** : `["header"]`                                  | < string > array                                  |
| **timeout**    <br>*optional*     | **Example** : `120000`                                      | int64                                             |
| **idleTimeout**    <br>*optional*| **Example** : `120000`                                      | int64                                             |
| **statefulSession**    <br>*optional*| Cookie based stateful session configuration for this route.                                    | [RouteStatefulSession](#routestatefulsession)                                             |
| **rateLimit**    <br>*optional*| Rate limit configuration name. | string              |

## Destination
| Name               | Description                                                      | Schema          |
|:-------------------|:-----------------------------------------------------------------|:----------------|
| **cluster**        | Microservice family name (does not include b/g version). **Example** : `"test-cluster"` | string          |
| **endpoint**       | Microservice address. Includes port and b/g version if appliable. **Example** : `"http://test-cluster-v1:8080"` | string          |
| **httpVersion**    | HTTP protocol version to be used. **Acceptable values:** : `1`, `2`. By default httpVersion is `1` | int32           |
| **tcpKeepalive**<br>*optional* | TCP keepalive settings for cluster upstream connections. | [TcpKeepalive](#tcpkeepalive) |

## TcpKeepalive
| Name               | Description              | Schema          |
|:-------------------|:-------------------------|:----------------|
| **probes**           | Maximum number of keepalive probes to send without response before deciding the connection is dead. Default is to use the OS level configuration (unless overridden, Linux defaults to 9.) | int32          |
| **time**          | The number of seconds a connection needs to be idle before keep-alive probes start being sent. Default is to use the OS level configuration (unless overridden, Linux defaults to 7200s (i.e., 2 hours.) | int32          |
| **interval**          | The number of seconds between keep-alive probes. Default is to use the OS level configuration (unless overridden, Linux defaults to 75s.) | int32          |

## HeaderDefinition
| Name               | Description              | Schema          |
|:-------------------|:-------------------------|:----------------|
| **name**           | **Example** : `"header"` | string          |
| **value**          | **Example** : `"value"`  | string          |

## DomainDeleteRequestV3
| Name               | Description                         | Schema           |
|:-------------------|:------------------------------------|:-----------------|
| **virtualService** | **Example** : `"virtual-service"`   | string           |
| **gateway**        | **Example** : `"gateway1"`          | string           |
| **domains**        | **Example** : `["domain1"]`         | < string > array |

## EndpointDeleteRequest

| Name                          | Description                                                  | Scheme                                              |
|:------------------------------|:-------------------------------------------------------------|:----------------------------------------------------|
| **endpoints**  <br>*optional* | **Example**: `["[endpointdeleteitem](#EndpointDeleteItem)"]` | < [EndpointDeleteItem](#EndpointDeleteItem) > array |
| **version**    <br>*optional* | **Example**: "string"                                        | string                                              |

## EndpointDeleteItem

| Name                        | Description             | Scheme |
|:----------------------------|:------------------------|:-------|
| **address**  <br>*optional* | **Example**: `"header"` | string |
| **port**     <br>*optional* | **Example**: `value`    | int32  |


## CompositePlatformStructure
| Name           | Description                                     | Schema          |
|:---------------|:------------------------------------------------|:-----------------|
| **baseline**   | **Example** : `"cloud-core"`                    | string           |
| **satellites** | **Example** : `["satellite-1", "satellite-2"]`  | < string > array |


## EndpointDeleteRequest

| Name                          | Description                                                  | Scheme                                              |
|:------------------------------|:-------------------------------------------------------------|:----------------------------------------------------|
| **endpoints**  <br>*optional* | **Example**: `["[endpointdeleteitem](#EndpointDeleteItem)"]` | < [EndpointDeleteItem](#EndpointDeleteItem) > array |
| **version**    <br>*optional* | **Example**: "string"                                        | string                                              |

## EndpointDeleteItem

| Name                        | Description             | Scheme |
|:----------------------------|:------------------------|:-------|
| **address**  <br>*optional* | **Example**: `"header"` | string |
| **port**     <br>*optional* | **Example**: `value`    | int32  |

## RouteStatefulSession

| Name                        | Description             | Scheme |
|:----------------------------|:------------------------|:-------|
| **enabled**  <br>*optional* | **Example**: `true`; **default**: `true` | bool   |
| **cookie**   <br>*optional* | Cookie for stateful session specification. Can be used to override stateful session cookie spec inhereted from cluster or endpoint. | [Cookie](#cookie)  |


## StatefulSession

| Name                        | Description             | Scheme |
|:----------------------------|:------------------------|:-------|
| **gateways** | **Example**: `["internal-gateway-service"]` | < string > array |
| **namespace**  <br>*optional* | **Example**: `satellite-1`; defaults to current control-plane namespace. | string |
| **enabled**  <br>*optional* | **Example**: `true`; **default**: `true` | bool   |
| **cluster**                 | Cluster (microservice family name). **Example**: `trace-service` | string |
| **version**  <br>*optional* | Blue-green version. **Example**: `v1`; **default**: current `ACTIVE` version. | string  |
| **port**  <br>*optional* | Endpoint port. If not specified, this configuration applies to the whole cluster. | integer  |
| **hostname**  <br>*optional* | Can be used to specify hostname of custom endpoint. **Example**: `trace-service-tls`. | string  |
| **cookie**   <br>*optional* | Cookie for stateful session specification. Empty `cookie` means that stateful session configuration for the resource needs to be deleted. | [Cookie](#cookie)  |


## RateLimit

| Name                        | Description             | Scheme |
|:----------------------------|:------------------------|:-------|
| **name** | Name of the rate limit configuration, used to reference this configuration by virtualService or route. **Example**: `"trace-service-get-ratelimit"` | string |
| **priority** <br>*optional* | Possible values are `"PRODUCT"` and `"PROJECT"`. Project configuration has higher priority. Default value is `"PRODUCT"`. | string |
| **limitRequestPerSecond** | RPS - requests per second. All the requests above the RPS are rejected with status code 429. **Example**: `10`. | integer |


## ServicesVersion

| Name                         | Description                                                                                                                                                                               | Scheme           |
|:-----------------------------|:------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|:-----------------|
| **version** <br>*optional*   | Blue-Green version name. For ROLLING_UPDATE "version" must be empty. **Example**: `"v1"`                                                                                                  | string           |
| **namespace** <br>*optional* | Only applicable for SANDBOX - in other cases "namespace" must be empty (control-plane will resolve its namespace automatically).                                                          | string           |
| **services**                 | List of services being deployed. Service names are the same as in application config.xml and in envs SERVICE_NAME provided by deployer. **Example**: `["trace-service", "echo-service"]`. | < string > array |


## VersionInRegistry

| Name         | Description                                        | Scheme                                            |
|:-------------|:---------------------------------------------------|:--------------------------------------------------|
| **version**  | Blue-Green version name. **Example**: `"v1"`       | string                                            |
| **stage**    | Stage of the blue-green version, e.g. `ACTIVE`.    | string                                            |
| **clusters** | List of clusters with their endpoints, if present. | < [ClusterInRegistry](#ClusterInRegistry) > array |

## ClusterInRegistry

| Name          | Description                                  | Scheme           |
|:--------------|:---------------------------------------------|:-----------------|
| **namespace** | Namespace in which microservice is deployed. | string           |
| **cluster**   | Service family name, e.g. `trace-service`.   | string           |
| **endpoints** | List of cluster endpoints, if present.       | < string > array |

## GatewayDeclaration

| Name                                  | Description                                                                                                             | Scheme  |
|:--------------------------------------|:------------------------------------------------------------------------------------------------------------------------|:--------|
| **name**                              | Gateway name, e.g. `my-composite-gateway`.                                                                              | string  |
| **gatewayType**                       | One of these constants: `ingress`, `mesh`, `egress`.                                                                    | string  |
| **allowVirtualHosts**  <br>*optional* | If explicitly set to `false`, then registering custom `hosts` for `virtualServices` will be forbidden for this gateway. | boolean |
| **exists**  <br>*optional*            | If explicitly set to `true`, then the request will be treated as deletion request.                                      | boolean |

## HttpFiltersConfigRequestV3

| Name                               | Description                                                                   | Scheme                                            |
|:-----------------------------------|:------------------------------------------------------------------------------|:--------------------------------------------------|
| **gateways**                       | Names of the gateways to which these filters configuration should be applied. | < string > array                                  |
| **wasmFilters**  <br>*optional*    | WASM filters configurations.                                                  | < [WasmFilter](#WasmFilter) > array |
| **extAuthzFilter**  <br>*optional* | ExtAuthz filter configuration.                                                | [ClusterInRegistry](#ClusterInRegistry)           |

## HttpFiltersDropConfigRequestV3

| Name                               | Description                                                                   | Scheme                              |
|:-----------------------------------|:------------------------------------------------------------------------------|:------------------------------------|
| **gateways**                       | Names of the gateways to which these filters configuration should be applied. | < string > array                    |
| **wasmFilters**  <br>*optional*    | Names of the WASM filters to drop.                                            | < [FilterDrop](#FilterDrop) > array |
| **extAuthzFilter**  <br>*optional* | ExtAuthz filter to drop.                                                      | [FilterDrop](#FilterDrop)           |


## WasmFilter

| Name                             | Description                                    | Scheme                          |
|:---------------------------------|:-----------------------------------------------|:--------------------------------|
| **name**                         | Unique filter name.                            | string                          |
| **url**                          | URL to wasm filter binary in artifact storage. | string                          |
| **sha256**                       | SHA256 checksum of the wasm filter binary.     | string                          |
| **tlsConfigName** <br>*optional* | Name of the TlsDef to be used by this filter.  | string                          |
| **timeout** <br>*optional*       | Timeout in seconds.                            | int64                           |
| **params** <br>*optional*        | Params to be passed to WASM filter.            | < <string>:<object> map > array |

## ExtAuthz

| Name                                 | Description                                                             | Scheme                      |
|:-------------------------------------|:------------------------------------------------------------------------|:----------------------------|
| **name**                             | Unique filter name.                                                     | string                      |
| **destination**                      | Destination - external authorization server to be used by filter.       | [Destination](#Destination) |
| **contextExtensions** <br>*optional* | Map of string keys and values that will be passed to filter by gateway. | < string > : < string > map |
| **timeout** <br>*optional*           | Timeout in milliseconds.                                                | int64                       |

## FilterDrop

| Name                             | Description                                    | Scheme                          |
|:---------------------------------|:-----------------------------------------------|:--------------------------------|
| **name**                         | Unique filter name.                            | string                          |


## ClusterTcpKeepalive

| Name                             | Description                                    | Scheme                          |
|:---------------------------------|:-----------------------------------------------|:--------------------------------|
| **clusterKey**                         | Full cluster key. Can be obtained from Mesh tab in cloud-administrator UI. **Example**: `my-service||my-service||8080` | string                          |
| **tcpKeepalive** <br>*optional*   | Tcp keepalive settings. If ommitted or null, tcp keepalive settings will be removed for the cluster. | [TcpKeepalive](#tcpkeepalive)                          |
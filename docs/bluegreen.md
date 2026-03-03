This documentation presents the Blue-Green Functionality.

# Overview

Blue-Green is about "zero downtime deployment" model. It makes sense when you want to upgrade your cloud application without any maintenance break.

A simple model of deployment means that you have one environment and when you update the application you show stub maintenance page to a user.

In our model we assume that an application deploys with two or more versions on the same cloud environment. You can chose which version of application you want to call using HTTP Header _X-Version_. The application which was deployed using another version of bundle is neighboring in the environment and it is called [**_Candidate_**](./glossary.md#candidate). The HTTP traffic can be directed to [**_Candidate_**](/docs/glossary.md#candidate) and [**_Legacy_**](./glossary.md#legacy) by only using requests with _X-Version_ header though, at the same time, user's traffic is directed to **_Active_** version of application.

# Control Plane modes

Control Plane can work in two modes: **VERSIONED** and **NAMESPACED**. So that if you want to know which one is set now you can call [appropriate endpoint](./api/control-plane-api.md#get-routing-mode-details). **VERSIONED** mode means that Control Plane can register routes only with the current [namespace](./glossary.md#namespace) where it was deployed, whereas **NAMESPACED** mode means that Control Plane can register routes only with [version](./glossary.md#version).

# Promote

This operation is intended for changing user traffic direction. When you finish testing **_Candidate_** you can direct all user's traffic to it. [**_Promote_**](./api/control-plane-api.md#promote) can be called manually using REST API, but usually this operation is called by _Deployment Job_. [**_Promote_**](./api/control-plane-api.md#promote) switches the current Active version to Legacy while  [**_Candidate_**](./glossary.md#candidate) to Active. Rules of traffic availability for Legacy is the same as for **_Candidate_** (using header).

>**CAUTION:** This operation makes a specified version of application ([**_Candidate_**](./glossary.md#candidate)) to be Active. Other Candidates (if only they are present) will be deleted and will not be available, inspite of micro-service instances of these versions being still running in the environment.


You cannot run Promote when:

* Your environment has no installed _Candidate_ (because you have nothing to promote)
* You attempt to promote that version of application which is not _Candidate_

## Changes of Control Plane storage

### State of persistence storage and routing after Blue-Green Deployment and calling Promote

Let us consider [**_Promote_**](./api/control-plane-api.md#promote) operation in terms of persistence storage and its changes.

Just imagine a situation when you have the only micro-service in cloud environment (which was deployed with Rolling model), and the Deployment Job (which was started by you) deployed another version of the same micro-service and to the same environment, but that deployment used Blue-Green deployment model. 
Let also each version of the micro-service has the only one route which was registered at **Control Plane** under one of gateway _node-group_ (for example, public-gateway-service). In this case we have the following data in DB storage. (Certainly, it is not a full representation of real data in DB, it is just a slice.)

_Routes table state in DB_

|ID|prefix| cluster_name|deployment_version|
|---|---|---|---|
|1|/api/v1/test|ms\|\|ms\|\|8080|v1|
|2|/api/v1/test|ms\|\|ms\|\|8080|v2|

_Clusters table state in DB_

|ID |Name
|---|---|
|1|ms\|\|ms\|\|8080|

_Endpoints table state in DB_

|ID | clusterId | address | port | deployment_version|
|---|---|---|---|---|
|1|1|ms-v1|8080|v1|
|2|1|ms-v2|8080|v2|

_DeploymentVersions table state in DB_

|version |stage|
|---|---|
|v1|ACTIVE|
|v2|CANDIDATE|

As we can see in the tables above, DB storage has two records of route, two records of deployment versions and two endpoints under the cluster. But there is a question how traffic routes in this case. The image below implies envoy routing.

![Routing model - v1 active](./images/routing-model-v1-active.png)

In the next step, we call [**_Promote_**](./api/control-plane-api.md#promote). In simple cases, when we have the same set of routes leading to two different versions of the micro-service, changes in DB are quite small.
Our simple case when calling [**_Promote_**](./api/control-plane-api.md#promote) operation just changes the state of _DeploymentVersion_ in DB, nothing else, while the routing model has changed cardinally.

_DeploymentVersions table state in DB_

|version |stage|
|---|---|
|v1|LEGACY|
|v2|ACTIVE|


![Routing model - v2 active](./images/routing-model-v2-active.png)

### Delete Candidates during Promote

In the next step, we deploy two versions of the same micro-service. They will be 'v3' and 'v4'. The data in DB will be the following:

_Routes table state in DB_


|ID | prefix | cluster_name | deployment_version|
|---|---|---|---|
|1|/api/v1/test|ms\|\|ms\|\|8080|v1|
|2|/api/v1/test|ms\|\|ms\|\|8080|v2|
|3|/api/v1/test|ms\|\|ms\|\|8080|v3|
|4|/api/v1/test|ms\|\|ms\|\|8080|v4|


_Clusters table state in DB_

|ID |Name|
|---|---|
|1|ms\|\|ms\|\|8080|

_Endpoints table state in DB_

|ID | clusterId | address | port | deployment_version|
|---|---|---|---|---|
|1|1|ms-v1|8080|v1|
|2|1|ms-v2|8080|v2|
|3|1|ms-v3|8080|v3|
|4|1|ms-v4|8080|v4|

_DeploymentVersions table state in DB_

|version |stage|
|---|---|
|v1||LEGACY|
|v2|ACTIVE|
|v3|CANDIDATE|
|v4||CANDIDATE|

![Routing model - v2 active, 2 candidates](./images/routing-model-v2-active-2-candidates.png)

As we can see from the image above, the whole traffic to candidates is possible only with the header, while the traffic is going to [Active](./glossary.md#active) [version](./glossary.md#version).

Well, we saw v4 of [**_Candidate_**](./glossary.md#candidate) are well and stable and we decide to [Promote](./api/control-plane-api.md#promote) it. Let us call [Promote](./api/control-plane-api.md#promote) REST API. Then we get the following result:

|ID | prefix | cluster_name | deployment_version|
|---|---|---|---|
|1|/api/v1/test|ms\|\|ms\|\|8080|v1|
|2|/api/v1/test|ms\|\|ms\|\|8080|v2|
|4|/api/v1/test|ms\|\|ms\|\|8080|v4|

_Clusters table state in DB_

|ID |Name|
|1|ms\|\|ms\|\|8080|

_Endpoints table state in DB_

|ID | clusterId | address | port | deployment_version|
|---|---|---|---|---|
|1|1|ms-v1|8080|v1|
|2|1|ms-v2|8080|v2|
|4|1|ms-v4|8080|v4|

_DeploymentVersions table state in DB_

|version |stage|
|---|---|
|v1|ARCHIVE|
|v2|LEGACY|
|v4||ACTIVE|

The routing model should be as follows.

![Routing model - v4 active](./images/routing-model-v4-active.png)

There we promoted v4 [**_Candidate_**](./glossary.md#candidate), and that means we have v4 as Active version, and user traffic is going to ms-v4 without any extra headers. At the same time v3 [**_Candidate_**](/docs/glossary.md#candidate) was deleted from DB at all (DeploymentVersion, Endpoints and Routes). Also you can see that v1 became Archive and it is present in DB storage but it is not participating in routing model.

 >**CAUTION:** If you send a request to API Gateway with header 'x-version: v1', that request will be routed to ms-v4 because this micro-service belongs to Active version and the default strategy of fallback scenario is routing to Active version.

# Rollback

This operation rollbacks to a previous version of application. [**_Rollback_*](./api/control-plane-api.md#rollback) is designed for a situation when you want to cancel [Promote](./api/control-plane-api.md#promote) operation and return back previous state of routing.

>**CAUTION:** If you have registered Candidates in Control Plane, that operation will delete all of them.

So we call [**_Rollback_**](./api/control-plane-api.md#rollback) and, according to previous state of db (_DeploymentVersions table stat in DB_ Table), this is what we get:

_DeploymentVersions table stat in DB after rollback_

|version |stage|
|---|---|
|v1||ARCHIVE|
|v2|ACTIVE|
|v4|CANDIDATE|

Rollback operation made v4 version as [**_Candidate_**](./glossary.md#candidate), while v2, which was LEGACY, became Active. But, as you can notice, the archive version has not changed, although you can think that it should become Legacy.
And the routing model should be as follows:

![Routing model rolled back](./images/routing-model-rolled-back.png)

>**WARNING:** The only Recovery operation can cast Archive version to Legacy/Active. This operation is not designed and implemented yet. That means you cannot change version which has became Archive.


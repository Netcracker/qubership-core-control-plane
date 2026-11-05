# Allowed and Forbidden Routes

## Legacy Cloud-Core Service Mesh Behavior

### Public, Private and Internal Routes

#### Legacy Behavior

In Legacy Cloud-Core Service Mesh libraries there was capability to specify route type in java annotation `@Route(type = ...)`: 

* PUBLIC - route that is accessible through Public, Private and Internal gateway;
* PRIVATE - route is only accessible through Private and Internal gateway;
* INTERNAL - route is accessible only through Internal gateway.

This behavior was achieved by always creating 3 routes for each annotated endpoint: allowed routes are regular routes and forbidden routes are routes with `directResponse#statusCode: 404`.

The same forbidden route could be created via declarative API via field `allowed: false`.

#### Migration to Istio Strategy

It is very tricky to fully replicate this behavior in Istio (see TBD: regex routes issue document link). However, having such complex schema for PUBLIC, PRIVATE and INTERNAL routes seems excessive and we do't want to repeat it in Istio. 

Instead, we assume: 

1. Every route should be registered in all API gateways: public, private, internal and all the gateways that can possibly appear in future (e.g. internet-gateway);
2. Every route should be allowed on private and internal gateways;
3. Cloud-Core should bring AuthorizationPolicy CR for public-gateway that works as deny-unless-permit (AuthorizationPolicy with action ALLOW and NO(empty) rules);
4. Each microservice brings its AuthorizationPolicy CR for public-gateway that allows only some specific subset of routes on public gateway. This AuthorizationPolicy CR should be written manually by microservice owners.
5. Since forbidden routes now emulated via AuthorizationPolicy, HTTPRoute CR no longer needs to register every endpoint separately - instead only REST controller's roots should become rules inside HTTPRoute CR.

### Deny Routes

#### Legacy Behavior

In Legacy Cloud-Core Service Mesh in declarative configuration there was also field `deny: true` that provides capability to overlap some allowed route with `directResponse#statusCode: 404`. Deny route has higher priority - it is used to forbid route that was allowed by some OOB configuration.

#### Migration to Istio Strategy

In Istio there is a capability to configure AuthorizationPolicy that can both extend and narrow the scope of routes allowed on public gateway OOB.

My plan:

1. Validate AuthorizationPolicy:
   - [x] deny-unless-permit on public gw;
   - [x] capability to allow more by project;
   - [x] capability to deny more by project;
2. Include AuthorizationPolicy for public in core-mesh-config;
3. Include AuthorizationPolicy CRs in each core service with public routes;
4. Sample microservice to test everything
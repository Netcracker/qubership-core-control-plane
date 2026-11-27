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

It is very tricky to fully replicate this behavior in Istio. A per-gateway PUBLIC / PRIVATE / INTERNAL
schema is more than Istio should copy.

Instead:

1. A route that was already on the public gateway is registered on the public, private, and internal
   gateways. A route that was only private or internal is registered on the private and internal
   gateways, and stays off the public gateway until someone has checked the AuthorizationPolicies
   that target that gateway. A project-level ALLOW (see [crs-example.yaml](crs-example.yaml)) can
   make a newly attached route publicly reachable even when the service has no policy of its own.
   Absence of a service-owned policy is not proof that the route is unreachable.
2. Every route should be allowed on the private and internal gateways. Do not forbid a PRIVATE or
   INTERNAL route, or an `allowed: false` route, on the private gateway. Those restrictions matter
   on the public gateway. A legacy `deny: true` may also matter on the gateway it came from,
   including the private gateway.
3. Cloud-Core brings an AuthorizationPolicy for `public-gateway` that is deny-unless-permit
   (`action: ALLOW` and no rules).
4. Each microservice brings its own AuthorizationPolicy for `public-gateway` that allows only the
   paths that must be public. Write that policy by hand. A generated draft is a separate step and
   runs only when someone explicitly asks for it.
5. Forbidden routes are not copied onto the HTTPRoute. The forbidden rule is omitted. Check whether
   a remaining rule and the public-gateway policy cover that path. Path regexes are not emitted:
   cut the route at the first path variable and emit a prefix matcher. Highlight that cut when it
   conflicts with another rule. Otherwise do not ask for a manual review of the cut.

### Deny Routes

#### Legacy Behavior

In Legacy Cloud-Core Service Mesh in declarative configuration there was also field `deny: true` that provides capability to overlap some allowed route with `directResponse#statusCode: 404`. Deny route has higher priority - it is used to forbid route that was allowed by some OOB configuration.

#### Migration to Istio Strategy

In Istio there is a capability to configure AuthorizationPolicy that can both extend and narrow the scope of routes allowed on public gateway by OOB AuthorizationPolicy.

# Steps 2–3 — Detect route definitions and extract fields

Part of the [`httproute-from-code`](SKILL.md) skill. Continue with Step 4 in [`SKILL.md`](SKILL.md) afterwards.

## Step 2 — Detect route definitions

### Go patterns

```go
// Struct literal inline
registrar.WithRoutes(
    routeregistration.Route{
        From:      "/api/v1/users",
        To:        "/users",
        RouteType: routeregistration.Public,
        Timeout:   30 * time.Second,
        Forbidden: false,
        Gateway:   "",
        Hosts:     []string{"api.company.com"},
    },
)

// Chained
routeregistration.NewRegistrar().
    WithRoutes(routeregistration.Route{...}).
    Register()

// Variable
r := routeregistration.Route{...}
registrar.WithRoutes(r)

// Slice spread
routes := []routeregistration.Route{...}
registrar.WithRoutes(routes...)

// Mesh
routeregistration.Route{
    From:      "/mesh",
    RouteType: routeregistration.Mesh,
    Gateway:   "mesh-gateway",
}

// Facade
routeregistration.Route{
    From:    "/facade",
    Gateway: "facade",
}
```

### Java patterns

> **IMPORTANT — Annotation-based routes are explicitly OUT OF SCOPE.**
> `@Route` / `@Gateway` class/method annotations are processed at compile time
> by `httproutes-generator-maven-plugin`. Do NOT
> extract routes from these annotations here — doing so would duplicate the
> plugin's output. Only extract routes from **`RouteEntry` builder/constructor
> call sites** as shown below.

```java
// Builder
RouteEntry.builder()
    .from("/api/v1/users")
    .to("/users")
    .type(RouteType.PUBLIC)
    .timeout(30000L)
    .allowed(true)
    .namespace("default")
    .gateway("my-gateway")
    .hosts(Set.of("api.company.com"))
    .build()

// Constructors — all variants
new RouteEntry("/api/v1/users", RouteType.PUBLIC)
new RouteEntry("/api/v1/users", RouteType.PUBLIC, 30000L)
new RouteEntry("/api/v1/users", RouteType.PUBLIC, "prod-namespace")
new RouteEntry("/api/v1/users", RouteType.PUBLIC, "prod-namespace", 30000L)
new RouteEntry("/api/v1/users", "/users", RouteType.PUBLIC)
new RouteEntry("/api/v1/users", "/users", RouteType.PUBLIC, 30000L)
new RouteEntry("/api/v1/users", "/users", RouteType.PUBLIC, "prod-namespace")
new RouteEntry("/api/v1/users", "/users", RouteType.PUBLIC, "prod-namespace", 30000L)

// Collections
List.of(new RouteEntry(...), RouteEntry.builder()...build())
routes.add(new RouteEntry(...))

// postRoutes call sites
processor.postRoutes(List.of(new RouteEntry(...)))
processor.postRoutes(microserviceUrl, routes)
```

---

## Step 3 — Extract fields

### Unified field table

| Field | Go source | Java source | Default |
|---|---|---|---|
| `from` | `From:` | `.from(...)` / 1st path arg | REQUIRED |
| `to` | `To:` | `.to(...)` / 2nd path arg | same as `from` |
| `routeType` | `RouteType:` | `.type(RouteType.X)` | Public / PUBLIC |
| `forbidden` | `Forbidden: true` | `.allowed(false)` | false |
| `namespace` | n/a (from config) | `.namespace(...)` / namespace arg | `default` |
| `timeout` | `Timeout:` | `.timeout(...)` / timeout arg | omit |
| `gateway` | `Gateway:` | `.gateway(...)` | derived |
| `hosts` | `Hosts:` | `.hosts(Set.of(...))` | omit |

### Java constructor disambiguation
3-arg `new RouteEntry(path, type, X)`:
- X is `Long` or numeric literal → timeout
- X is `String` → namespace

4-arg `new RouteEntry(from, to, type, X)`:
- X is `Long` or numeric literal → timeout
- X is `String` → namespace

### RouteType normalization

| Go | Java | Canonical |
|---|---|---|
| `routeregistration.Public` | `RouteType.PUBLIC` | `Public` |
| `routeregistration.Private` | `RouteType.PRIVATE` | `Private` |
| `routeregistration.Internal` | `RouteType.INTERNAL` | `Internal` |
| `routeregistration.Mesh` | `RouteType.MESH` | `Mesh` |
| n/a | `RouteType.FACADE` | `Facade` |

---


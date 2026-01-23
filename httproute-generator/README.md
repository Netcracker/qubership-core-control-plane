# HTTPRoute Generator Maven Plugin

A Maven plugin that automatically generates Kubernetes Gateway API HTTPRoute resources from Spring Boot and Quarkus REST API annotations.

## Overview

The HTTPRoute Generator scans your Java application's compiled classes for REST endpoint annotations (Spring `@RequestMapping`, `@GetMapping`, etc., or Quarkus `@Path`) and automatically generates HTTPRoute Custom Resources for Kubernetes Gateway API. This eliminates manual YAML management and ensures your routing configuration stays in sync with your actual API endpoints.

### Key Features

- **Automatic Route Discovery**: Scans Spring Boot and Quarkus REST annotations
- **Multi-Module Maven Projects**: Aggregates routes from all reactor modules
- **Helm Template Ready**: Generates YAML with Helm template placeholders

## Installation

Add the plugin to your `pom.xml`:

```xml
<build>
    <plugins>
        <plugin>
            <groupId>org.qubership.routes</groupId>
            <artifactId>gateway-plugin</artifactId>
            <version>1.0.0</version>
            <executions>
                <execution>
                    <goals>
                        <goal>generate-routes</goal>
                    </goals>
                </execution>
            </executions>
            <configuration>
                <packages>
                    <package>com.example</package>
                </packages>
                <servicePort>8080</servicePort>
                <outputFile>gateway-httproutes.yaml</outputFile>
                <backendRefVal>{{ .Values.DEPLOYMENT_RESOURCE_NAME }}</backendRefVal>
            </configuration>
        </plugin>
    </plugins>
</build>
```

## Configuration Parameters

| Parameter       | Type     | Default                                  | Description                                      |
|-----------------|----------|------------------------------------------|--------------------------------------------------|
| `packages`      | String[] | `com.netcracker`                         | Package prefixes to scan for annotations         |
| `servicePort`   | int      | `8080`                                   | Backend service port                             |
| `outputFile`    | String   | `gateway-httproutes.yaml`                | Output file path (relative to project root)      |
| `backendRefVal` | String   | `{{ .Values.DEPLOYMENT_RESOURCE_NAME }}` | Backend reference name (supports Helm templates) |

## Usage

### Basic Example - Spring Boot

Annotate your REST controllers:

```java
@RestController
@RequestMapping("/api/users")
public class UserController {
    
    @GetMapping("/{id}")
    public User getUser(@PathVariable String id) {
        // ...
    }
    
    @PostMapping
    public User createUser(@RequestBody User user) {
        // ...
    }
}
```

Run Maven build:

```bash
mvn clean compile
```

Generated `gateway-httproutes.yaml`:

```yaml
# -----------------------------------------------------------------------------
# THIS FILE WAS AUTOMATICALLY GENERATED — DO NOT EDIT.
# Any changes will be overwritten during the next build.
# Modify source annotations and regenerate using route generation maven plugin.
# -----------------------------------------------------------------------------

{{ if .Values.ISTIO_ENABLED }}
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: {{ .Values.SERVICE_NAME }}-internal
  namespace: {{ .Values.NAMESPACE }}
  labels:
    app.kubernetes.io/managed-by: {{ .Values.MANAGED_BY }}
    app.kubernetes.io/name: {{ .Values.SERVICE_NAME }}
    app.kubernetes.io/part-of: {{ .Values.APPLICATION_NAME }}
    app.kubernetes.io/processed-by-operator: istiod
    deployer.cleanup/allow: "true"
    deployment.netcracker.com/sessionId: {{ .Values.DEPLOYMENT_SESSION_ID }}
spec:
  parentRefs:
  - name: internal-gateway-service
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /api/users
    backendRefs:
    - name: {{ .Values.DEPLOYMENT_RESOURCE_NAME }}
      port: 8080
  - matches:
    - path:
        type: RegularExpression
        value: /api/users/([^/]+)
    backendRefs:
    - name: {{ .Values.DEPLOYMENT_RESOURCE_NAME }}
      port: 8080
{{ end }}
```

### Using @Route Annotation for Gateway Types

Control which gateway exposes your routes:

```java
import com.netcracker.cloud.routesregistration.common.annotation.Route;

@RestController
@RequestMapping("/api/admin")
@Route(type = Route.Type.PRIVATE) // Only accessible via private gateway
public class AdminController {
    
    @GetMapping("/users")
    public List<User> listUsers() {
        // ...
    }
}

@RestController
@RequestMapping("/api/public")
@Route(type = Route.Type.PUBLIC) // Accessible via public, private, and internal gateways
public class PublicController {
    
    @GetMapping("/status")
    public Status getStatus() {
        // ...
    }
}
```

**Gateway Type Hierarchy:**

- `INTERNAL`: Only internal-gateway-service
- `PRIVATE`: private-gateway-istio + internal-gateway-service
- `PUBLIC`: public-gateway-istio + private-gateway-istio + internal-gateway-service
- `FACADE`: Custom service-specific gateway

### Path Rewriting with @Gateway

Map external gateway paths to internal service paths:

```java
import com.netcracker.cloud.routesregistration.common.annotation.Gateway;

@RestController
@RequestMapping("/my-service/users") // Internal service path
@Gateway("/api/v1/users")          // External gateway path
public class UserController {
    
    @GetMapping("/{id}")
    public User getUser(@PathVariable String id) {
        // Accessible at gateway: /api/v1/users/{id}
        // Rewrites to service: /my-service/users/{id}
    }
}
```

Generated HTTPRoute includes URL rewrite filter:

```yaml
rules:
- matches:
  - path:
      type: RegularExpression
      value: /api/v1/users/([^/]+)
  filters:
  - type: URLRewrite
    urlRewrite:
      path:
        type: ReplacePrefixMatch
        replacePrefixMatch: /my-service/users
  backendRefs:
  - name: {{ .Values.DEPLOYMENT_RESOURCE_NAME }}
    port: 8080
```

### Quarkus Support

Works with Quarkus JAX-RS annotations:

```java
import jakarta.ws.rs.*;

@Path("/api/products")
public class ProductResource {
    
    @GET
    @Path("/{id}")
    public Product getProduct(@PathParam("id") String id) {
        // ...
    }
}
```

### Multi-Module Maven Projects

The plugin runs as an aggregator and collects routes from all modules:

```
my-app/
├── pom.xml (aggregator, plugin configured here)
├── user-service/
│   └── src/main/java/.../UserController.java
├── order-service/
│   └── src/main/java/.../OrderController.java
└── gateway-httproutes.yaml (generated with all routes)
```

## Build Lifecycle

The plugin executes during the `process-classes` phase:

```
compile → process-classes → [generate-routes] → package
```

To run independently:

```bash
mvn org.qubership.routes:gateway-plugin:generate-routes
```

## Troubleshooting

### No routes generated

**Problem**: Empty output file

**Solution**: Ensure classes are compiled and packages match:
```bash
mvn clean compile
# Check outputDirectory exists
ls target/classes
```

### Wrong package scanned

**Problem**: Controllers not found

**Solution**: Update `packages` configuration:
```xml
<configuration>
    <packages>
        <package>com.example</package>
        <package>org.mycompany</package>
    </packages>
</configuration>
```

### Path variables not converted to regex

**Problem**: `{id}` appears literally in path

**Solution**: This is expected behavior - the plugin converts `{variable}` to `([^/]+)` regex automatically.

### Routes from parent classes missing

**Problem**: Inherited endpoints not detected

**Solution**: The scanner automatically processes superclasses. Ensure parent class is in scanned packages.

### Helm template syntax issues

**Problem**: Generated YAML has invalid Helm templates

**Solution**: The plugin wraps output with:
```yaml
{{ if .Values.ISTIO_ENABLED }}
# ... routes ...
{{ end }}
```

Ensure your Helm chart defines these values.

## Advanced Configuration

### Custom Backend Reference

```xml
<configuration>
    <backendRefVal>my-service-backend</backendRefVal>
</configuration>
```

Generates:

```yaml
backendRefs:
- name: my-service-backend
  port: 8080
```

### Custom Output Location

```xml
<configuration>
    <outputFile>k8s/routes/gateway-routes.yaml</outputFile>
</configuration>
```

## Dependencies

The plugin uses:

- **ClassGraph**: For bytecode scanning and annotation discovery
- **Jackson**: For YAML serialization
- **Maven Plugin API**: For Maven integration

No runtime dependencies required in your application.

## Related Resources

- [Kubernetes Gateway API](https://gateway-api.sigs.k8s.io/)
- [Spring Web Annotations](https://docs.spring.io/spring-framework/docs/current/reference/html/web.html#mvc-ann-requestmapping)
- [Quarkus REST](https://quarkus.io/guides/rest-json)
- [Istio Gateway](https://istio.io/latest/docs/reference/config/networking/gateway/)

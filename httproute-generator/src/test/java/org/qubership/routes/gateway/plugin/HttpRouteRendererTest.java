package org.qubership.routes.gateway.plugin;

import org.junit.jupiter.api.Test;

import java.util.Set;

import static org.junit.jupiter.api.Assertions.assertTrue;

class HttpRouteRendererTest {

    @Test
    void generatesYamlWithMatchesRewritesAndTimeouts() {
        Set<HttpRoute> routes = Set.of(
                new HttpRoute("/api", HttpRoute.Type.INTERNAL),
                new HttpRoute("/svc", "/gateway", HttpRoute.Type.PUBLIC, 5_000),
                new HttpRoute("/items/{id}", HttpRoute.Type.PRIVATE)
        );

        String yaml = new HttpRouteRenderer("{{ CustomBackendRef }}").generateHttpRoutesYaml(8081, routes);

        assertTrue(yaml.contains("HTTPRoute"));
        assertTrue(yaml.contains("ReplacePrefixMatch"));
        assertTrue(yaml.contains("request: \"5s\""));
        assertTrue(yaml.contains("RegularExpression"));
        assertTrue(yaml.contains("items/([^/]+)"));
        assertTrue(yaml.contains("{{ CustomBackendRef }}"));
    }
}

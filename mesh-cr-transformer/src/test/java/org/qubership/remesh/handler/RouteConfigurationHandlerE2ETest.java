package org.qubership.remesh.handler;

import com.fasterxml.jackson.databind.JsonNode;
import org.junit.jupiter.api.Test;
import org.qubership.remesh.dto.gatewayapi.HttpRoute;
import org.qubership.remesh.serialization.RawYamlExtractor;
import org.qubership.remesh.serialization.YamlPreprocessor;
import org.qubership.remesh.util.ObjectMapperProvider;

import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;

class RouteConfigurationHandlerE2ETest {

    private static final String YAML = """
            apiVersion: core.netcracker.com/v1
            kind: Mesh
            subKind: RouteConfiguration
            metadata:
              name: {{ .Values.SERVICE_NAME }}-mesh-routes
              namespace: "{{ .Values.NAMESPACE }}"
              labels:
                {{ include "labels.common" . | nindent 4 | trim }}
                deployer.cleanup/allow: "true"
                app.kubernetes.io/processed-by-operator: core-operator
            spec:
              virtualServices:
                - name: "{{ .Values.SERVICE_NAME }}"
                  hosts: ["{{ .Values.SERVICE_NAME }}"]
                  routeConfiguration:
                    version: "{{ .Values.DEPLOYMENT_VERSION }}"
                    routes:
                      - destination:
                          cluster: "{{ .Values.SERVICE_NAME }}"
                          endpoint: http://{{ .Values.DEPLOYMENT_RESOURCE_NAME }}:8080
                        rules:
                          - match:
                              prefix: /actuator
                            allowed: false
                          - match:
                              prefix: /v1/test
                              headerMatchers:
                                - name: "Authorization"
                                  presentMatch: true
                            allowed: true
                            prefixRewrite: /api/v1/test
                            addHeaders:
                              - name: "NewHeader"
                                value: "NewHeaderValue"
                            removeHeaders: ["Cookie", "Set-Cookie"]
            """;

    @Test
    void convertsRouteConfigurationToHttpRoute() {
        YamlPreprocessor preprocessor = new YamlPreprocessor(ObjectMapperProvider.getMapper());
        RawYamlExtractor extractor = new RawYamlExtractor();

        JsonNode node = preprocessor.readAsJsonNode(YAML);
        assertNotNull(node);

        String rawMetadata = extractor.extractMetadata(YAML);
        MeshResourceFragment fragment = MeshResourceFragment.create(0, node, rawMetadata);

        RouteConfigurationHandler handler = new RouteConfigurationHandler();
        List<Resource> resources = handler.handle(fragment, Map.of());

        assertEquals(1, resources.size());
        HttpRoute httpRoute = (HttpRoute) resources.getFirst();

        // Metadata is preserved in rawMetadata, not deserialized
        assertNotNull(httpRoute.getRawMetadata());

        HttpRoute.HttpRouteSpec spec = httpRoute.getSpec();
        assertNotNull(spec);
        assertEquals(List.of("{{ .Values.SERVICE_NAME }}"), spec.getHostnames());
        assertNotNull(spec.getParentRefs());
        assertEquals(1, spec.getRules().size());

        HttpRoute.Rule rule = spec.getRules().getFirst();
        assertEquals(1, rule.getMatches().size());
        HttpRoute.Match match = rule.getMatches().getFirst();
        assertEquals(HttpRoute.PathMatchType.PathPrefix, match.getPath().getType());
        assertEquals("/v1/test", match.getPath().getValue());
        assertNotNull(match.getHeaders());
        assertEquals(1, match.getHeaders().size());
        HttpRoute.HeaderMatch headerMatch = match.getHeaders().getFirst();
        assertEquals("Authorization", headerMatch.getName());
        assertEquals(HttpRoute.HeaderMatchType.RegularExpression, headerMatch.getType());
        assertEquals(".*", headerMatch.getValue());

        assertEquals(2, rule.getFilters().size());
        HttpRoute.Filter urlRewrite = rule.getFilters().getFirst();
        assertEquals(HttpRoute.FilterType.URLRewrite, urlRewrite.getType());
        assertNotNull(urlRewrite.getUrlRewrite());
        assertNotNull(urlRewrite.getUrlRewrite().getPath());
        assertEquals(HttpRoute.PathRewriteType.ReplacePrefixMatch, urlRewrite.getUrlRewrite().getPath().getType());
        assertEquals("/api/v1/test", urlRewrite.getUrlRewrite().getPath().getReplacePrefixMatch());

        HttpRoute.Filter headersFilter = rule.getFilters().get(1);
        assertEquals(HttpRoute.FilterType.RequestHeaderModifier, headersFilter.getType());
        assertNotNull(headersFilter.getRequestHeaderModifier());
        assertEquals(1, headersFilter.getRequestHeaderModifier().getAdd().size());
        HttpRoute.Header addedHeader = headersFilter.getRequestHeaderModifier().getAdd().getFirst();
        assertEquals("NewHeader", addedHeader.getName());
        assertEquals("NewHeaderValue", addedHeader.getValue());
        assertEquals(List.of("Cookie", "Set-Cookie"), headersFilter.getRequestHeaderModifier().getRemove());

        assertEquals(1, rule.getBackendRefs().size());
        HttpRoute.BackendRef backendRef = rule.getBackendRefs().getFirst();
        assertEquals("Service", backendRef.getKind());
        assertEquals("{{ .Values.DEPLOYMENT_RESOURCE_NAME }}", backendRef.getName());
        assertEquals("8080", backendRef.getPort());
    }
}
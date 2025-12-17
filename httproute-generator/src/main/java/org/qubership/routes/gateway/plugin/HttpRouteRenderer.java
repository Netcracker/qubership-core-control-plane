package org.qubership.routes.gateway.plugin;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import java.util.stream.Collectors;

public class HttpRouteRenderer {

    private static final ObjectMapper YAML_MAPPER = yamlMapper();

    private static final long SECOND = 1_000;
    private static final long MINUTE = 60_000;
    private static final long HOUR = 3_600_000;

    private final String backendRefVal;

    public HttpRouteRenderer(String backendRefVal) {
        this.backendRefVal = backendRefVal;
    }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public record HTTPRouteResource(String apiVersion, String kind, Metadata metadata, Spec spec) {
        public record Metadata(String name, String namespace, Map<String, String> labels) {
        }

        public record Spec(Set<ParentRef> parentRefs, List<Rule> rules) {

            public record ParentRef(String name) {
            }

            public record Rule(
                    List<Match> matches,
                    List<Filter> filters,
                    List<BackendRef> backendRefs,
                    Timeouts timeouts
            ) {
                public record Timeouts(String request) {
                }
            }

            public record Match(Path path) {
                public record Path(String type, String value) {
                }
            }

            public record Filter(String type, UrlRewrite urlRewrite) {
                public record UrlRewrite(Path path) {
                    public record Path(String type, String replacePrefixMatch) {
                    }
                }
            }

            public record BackendRef(String name, Integer port) {
            }
        }
    }

    public String generateHttpRoutesYaml(int servicePort, Set<HttpRoute> httpRoutes) {
        List<HTTPRouteResource> routes = createHttpRoutes(servicePort, httpRoutes);

        return routes.stream()
                .map(HttpRouteRenderer::writeYaml)
                .collect(Collectors.joining());
    }

    private static HTTPRouteResource.Spec.Match.Path convertSpringPathToHttpRoutePath(String springPath) {
        if (springPath == null || springPath.isEmpty()) {
            return new HTTPRouteResource.Spec.Match.Path("PathPrefix", "/");
        }

        // If path contains Spring placeholders {variable}, use regex
        if (springPath.contains("{")) {
            return new HTTPRouteResource.Spec.Match.Path("RegularExpression", normalizePath(springPath.replaceAll("\\{([^/]+?)}", "([^/]+)")));
        } else {
            return new HTTPRouteResource.Spec.Match.Path("PathPrefix", normalizePath(springPath));
        }
    }

    private static String normalizePath(String path) {
        if (path == null || path.isEmpty()) {
            return "/";
        }
        if (!path.startsWith("/")) {
            return "/" + path;
        }
        return path;
    }

    public static String formatDuration(long ms) {
        if (ms < SECOND) {
            return ms + "ms";
        }
        if (ms % HOUR == 0) {
            return (ms / HOUR) + "h";
        }
        if (ms % MINUTE == 0) {
            return (ms / MINUTE) + "m";
        }
        if (ms % SECOND == 0) {
            return (ms / SECOND) + "s";
        }
        return ms + "ms";
    }

    private List<HTTPRouteResource> createHttpRoutes(int servicePort, Set<HttpRoute> httpRoutes) {
        Map<HttpRoute.Type, List<HttpRoute>> routesByType = httpRoutes
                .stream()
                .collect(Collectors.groupingBy(HttpRoute::type));

        return routesByType.entrySet().stream()
                .map(entry -> toResource(entry.getKey(), entry.getValue(), servicePort))
                .toList();
    }

    private HTTPRouteResource toResource(HttpRoute.Type type, List<HttpRoute> routes, int servicePort) {
        HTTPRouteResource.Metadata metadata =
                new HTTPRouteResource.Metadata(
                        "{{ .Values.SERVICE_NAME }}-" + type.name().toLowerCase(),
                        "{{ .Values.NAMESPACE }}",
                        Map.of(
                                "app.kubernetes.io/name", "{{ .Values.SERVICE_NAME }}",
                                "app.kubernetes.io/part-of", "{{ .Values.APPLICATION_NAME }}",
                                "app.kubernetes.io/managed-by", "{{ .Values.MANAGED_BY }}",
                                "deployment.netcracker.com/sessionId", "{{ .Values.DEPLOYMENT_SESSION_ID }}",
                                "deployer.cleanup/allow", "true",
                                "app.kubernetes.io/processed-by-operator", "istiod"
                        )
                );
        HTTPRouteResource.Spec.ParentRef parentRef = new HTTPRouteResource.Spec.ParentRef(type.gatewayName());
        List<HTTPRouteResource.Spec.BackendRef> backendRefs =
                List.of(new HTTPRouteResource.Spec.BackendRef(this.backendRefVal, servicePort));

        List<HTTPRouteResource.Spec.Rule> ruleList = new ArrayList<>(routes.size());
        for (HttpRoute route : routes) {
            ruleList.add(toRule(route, backendRefs));
        }

        HTTPRouteResource.Spec spec = new HTTPRouteResource.Spec(Set.of(parentRef), ruleList);
        return new HTTPRouteResource("gateway.networking.k8s.io/v1", "HTTPRoute", metadata, spec);
    }

    private static HTTPRouteResource.Spec.Rule toRule(HttpRoute route, List<HTTPRouteResource.Spec.BackendRef> backendRefs) {
        HTTPRouteResource.Spec.Match match = new HTTPRouteResource.Spec.Match(
                convertSpringPathToHttpRoutePath(route.gatewayPath()));

        List<HTTPRouteResource.Spec.Filter> filters = buildRewriteFilter(route);
        HTTPRouteResource.Spec.Rule.Timeouts timeouts = buildTimeout(route);

        return new HTTPRouteResource.Spec.Rule(
                List.of(match),
                filters,
                backendRefs,
                timeouts
        );
    }

    private static List<HTTPRouteResource.Spec.Filter> buildRewriteFilter(HttpRoute route) {
        String normalizedGateway = normalizePath(route.gatewayPath());
        String normalizedService = normalizePath(route.path());
        if (Objects.equals(normalizedGateway, normalizedService)) {
            return null;
        }

        HTTPRouteResource.Spec.Filter.UrlRewrite.Path rewritePath =
                new HTTPRouteResource.Spec.Filter.UrlRewrite.Path("ReplacePrefixMatch", normalizedService);
        HTTPRouteResource.Spec.Filter.UrlRewrite urlRewrite =
                new HTTPRouteResource.Spec.Filter.UrlRewrite(rewritePath);
        HTTPRouteResource.Spec.Filter filter =
                new HTTPRouteResource.Spec.Filter("URLRewrite", urlRewrite);
        return List.of(filter);
    }

    private static HTTPRouteResource.Spec.Rule.Timeouts buildTimeout(HttpRoute route) {
        if (route.timeout() <= 0) {
            return null;
        }
        return new HTTPRouteResource.Spec.Rule.Timeouts(formatDuration(route.timeout()));
    }

    private static String writeYaml(HTTPRouteResource route) {
        try {
            return YAML_MAPPER.writeValueAsString(route);
        } catch (Exception e) {
            throw new RuntimeException("Failed to serialize HTTPRouteResource to YAML", e);
        }
    }

    private static ObjectMapper yamlMapper() {
        ObjectMapper mapper = new ObjectMapper(new YAMLFactory());
        mapper.setSerializationInclusion(JsonInclude.Include.NON_NULL);
        return mapper;
    }
}

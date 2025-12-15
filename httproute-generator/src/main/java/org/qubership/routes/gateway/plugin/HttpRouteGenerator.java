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

public class HttpRouteGenerator {

    private static final ObjectMapper YAML_MAPPER = yamlMapper();

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public record HTTPRouteResource(String apiVersion, String kind, Metadata metadata, Spec spec) {
        public record Metadata(String name) {
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

    public static String generateHttpRoutesYaml(int servicePort, Set<HttpRoute> httpRoutes) {
        List<HTTPRouteResource> routes = createHttpRoutes(servicePort, httpRoutes);

        return routes.stream()
                .map(HttpRouteGenerator::writeYaml)
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
        if (ms < 1000) {
            return ms + "ms";
        }
        if (ms % 3_600_000 == 0) {
            return (ms / 3_600_000) + "h";
        }
        if (ms % 60_000 == 0) {
            return (ms / 60_000) + "m";
        }
        if (ms % 1000 == 0) {
            return (ms / 1000) + "s";
        }
        return ms + "ms";
    }

    private static List<HTTPRouteResource> createHttpRoutes(int servicePort, Set<HttpRoute> httpRoutes) {
        Map<HttpRoute.Type, List<HttpRoute>> routesByType = httpRoutes
                .stream()
                .collect(Collectors.groupingBy(HttpRoute::type));

        return routesByType.entrySet().stream()
                .map(entry -> toResource(entry.getKey(), entry.getValue(), servicePort))
                .toList();
    }

    private static HTTPRouteResource toResource(HttpRoute.Type type, List<HttpRoute> routes, int servicePort) {
        HTTPRouteResource.Metadata metadata =
                new HTTPRouteResource.Metadata("{{ .Values.SERVICE_NAME }}-" + type.name().toLowerCase());
        HTTPRouteResource.Spec.ParentRef parentRef = new HTTPRouteResource.Spec.ParentRef(type.gatewayName());
        List<HTTPRouteResource.Spec.BackendRef> backendRefs =
                List.of(new HTTPRouteResource.Spec.BackendRef("{{ .Values.SERVICE_NAME }}", servicePort));

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

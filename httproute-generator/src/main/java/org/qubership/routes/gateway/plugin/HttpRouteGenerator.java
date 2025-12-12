package org.qubership.routes.gateway.plugin;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import java.util.stream.Collectors;

public class HttpRouteGenerator {

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

    private static List<HTTPRouteResource> createHttpRoutes(int servicePort, Set<HttpRoute> httpRoutes) {
        Map<HttpRoute.Type, List<HttpRoute>> routesByType = httpRoutes.stream().collect(Collectors.groupingBy(HttpRoute::type));

        return routesByType.entrySet().stream().map(entry -> {
            HttpRoute.Type type = entry.getKey();
            List<HttpRoute> routes = entry.getValue();

            HTTPRouteResource.Metadata metadata = new HTTPRouteResource.Metadata("{{ .Values.SERVICE_NAME }}-" + type.name().toLowerCase());

            HTTPRouteResource.Spec.ParentRef parentRef = new HTTPRouteResource.Spec.ParentRef(type.gatewayName());

            List<HTTPRouteResource.Spec.Rule> ruleList = new ArrayList<>();
            List<HTTPRouteResource.Spec.BackendRef> backendRefs =
                    List.of(new HTTPRouteResource.Spec.BackendRef("{{ .Values.SERVICE_NAME }}", servicePort));

            for (HttpRoute route : routes) {
                // match uses the externally visible (gateway) path
                var matchPath = convertSpringPathToHttpRoutePath(route.gatewayPath());
                HTTPRouteResource.Spec.Match match = new HTTPRouteResource.Spec.Match(matchPath);

                List<HTTPRouteResource.Spec.Filter> filters = null;
                // If gateway path differs from service path, add URLRewrite ReplacePrefixMatch
                String normalizedGateway = normalizePath(route.gatewayPath());
                String normalizedService = normalizePath(route.path());
                if (!Objects.equals(normalizedGateway, normalizedService)) {
                    String rewriteTarget = normalizedService;
                    HTTPRouteResource.Spec.Filter.UrlRewrite.Path rewritePath =
                            new HTTPRouteResource.Spec.Filter.UrlRewrite.Path("ReplacePrefixMatch", rewriteTarget);
                    HTTPRouteResource.Spec.Filter.UrlRewrite urlRewrite =
                            new HTTPRouteResource.Spec.Filter.UrlRewrite(rewritePath);
                    HTTPRouteResource.Spec.Filter filter =
                            new HTTPRouteResource.Spec.Filter("URLRewrite", urlRewrite);
                    filters = List.of(filter);
                }

                HTTPRouteResource.Spec.Rule.Timeouts timeouts = null;
                if (route.timeout() > 0) {
                    timeouts = new HTTPRouteResource.Spec.Rule.Timeouts(formatDuration(route.timeout()));
                }

                ruleList.add(new HTTPRouteResource.Spec.Rule(
                        List.of(match),
                        filters,
                        backendRefs,
                        timeouts
                ));
            }

            HTTPRouteResource.Spec spec = new HTTPRouteResource.Spec(Set.of(parentRef), ruleList);

            return new HTTPRouteResource("gateway.networking.k8s.io/v1", "HTTPRoute", metadata, spec);
        }).toList();
    }

    public static String generateHttpRoutesYaml(int servicePort, Set<HttpRoute> httpRoutes) {
        List<HTTPRouteResource> routes = createHttpRoutes(servicePort, httpRoutes);

        ObjectMapper mapper = new ObjectMapper(new YAMLFactory());
        mapper.setSerializationInclusion(JsonInclude.Include.NON_NULL);
        return routes.stream().map(route -> {
            try {
                return mapper.writeValueAsString(route);
            } catch (JsonProcessingException e) {
                throw new RuntimeException(e);
            }
        }).collect(Collectors.joining());
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

}

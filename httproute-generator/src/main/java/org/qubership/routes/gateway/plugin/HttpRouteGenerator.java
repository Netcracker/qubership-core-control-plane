package org.qubership.routes.gateway.plugin;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;

import java.util.*;
import java.util.stream.Collectors;

public class HttpRouteGenerator {

    private static final int BACKEND_SERVICE_PORT = 8080;
    private static final String FACADE_GATEWAY_NAME = "facade-gateway-istio";
    private static final String INTERNAL_GATEWAY_NAME = "internal-gateway-istio";
    private static final String PRIVATE_GATEWAY_NAME = "private-gateway-istio";
    private static final String PUBLIC_GATEWAY_NAME = "public-gateway-istio";

    @JsonInclude(JsonInclude.Include.NON_NULL)
    static class HTTPRouteResource {
        public String apiVersion = "gateway.networking.k8s.io/v1";
        public String kind = "HTTPRoute";
        public Metadata metadata = new Metadata();
        public Spec spec = new Spec();

        static class Metadata {
            public String name;
        }

        static class Spec {
            public Set<ParentRef> parentRefs = new HashSet<>();
            public List<Rule> rules = new ArrayList<>();

            static class ParentRef {
                public String name;

                @Override
                public boolean equals(Object o) {
                    if (this == o) {
                        return true;
                    }
                    if (o == null || getClass() != o.getClass()) {
                        return false;
                    }
                    ParentRef parentRef = (ParentRef) o;
                    return Objects.equals(name, parentRef.name);
                }

                @Override
                public int hashCode() {
                    return Objects.hash(name);
                }
            }

            static class Rule {
                public List<Match> matches = new ArrayList<>();
            }

            static class Match {
                public Path path = new Path();
                public List<ForwardTo> forwardTo = new ArrayList<>();

                static class Path {
                    public String type = "PathPrefix";
                    public String value;
                }

                static class ForwardTo {
                    public TargetRef targetRef = new TargetRef();

                    static class TargetRef {
                        public String kind = "Service";
                        public String name;
                        public int port = BACKEND_SERVICE_PORT;
                    }
                }
            }
        }
    }

    private static List<HTTPRouteResource> createHttpRoutes(String serviceName, Set<HttpRoute> httpRoutes) {
        // Group routes by RouteType
        Map<RouteType, List<HttpRoute>> routesByType = httpRoutes.stream()
                .collect(Collectors.groupingBy(HttpRoute::getType));

        // Create one HTTPRouteResource per RouteType
        return routesByType.entrySet().stream()
                .map(entry -> {
                    RouteType type = entry.getKey();
                    List<HttpRoute> routes = entry.getValue();

                    HTTPRouteResource routeResource = new HTTPRouteResource();
                    routeResource.metadata.name = serviceName + "-" + type.name().toLowerCase();
                    HTTPRouteResource.Spec.ParentRef parentRef = new HTTPRouteResource.Spec.ParentRef();
                    parentRef.name = getGatewayName(type);
                    routeResource.spec.parentRefs.add(parentRef);
                    HTTPRouteResource.Spec.Rule rule = new HTTPRouteResource.Spec.Rule();

                    for (HttpRoute httpRoute : routes) {
                        HTTPRouteResource.Spec.Match match = new HTTPRouteResource.Spec.Match();
                        match.path = convertSpringPathToHttpRoutePath(httpRoute.getPath());

                        HTTPRouteResource.Spec.Match.ForwardTo forwardTo = new HTTPRouteResource.Spec.Match.ForwardTo();
                        forwardTo.targetRef.name = serviceName;
                        match.forwardTo.add(forwardTo);

                        rule.matches.add(match);
                    }

                    routeResource.spec.rules.add(rule);

                    return routeResource;
                })
                .toList();
    }

    public static String generateHttpRoutesYaml(String serviceName, Set<HttpRoute> httpRoutes) {
        List<HTTPRouteResource> routes = createHttpRoutes(serviceName, httpRoutes);

        ObjectMapper mapper = new ObjectMapper(new YAMLFactory());
        mapper.setSerializationInclusion(JsonInclude.Include.NON_NULL);
        return routes.stream()
                .map(route -> {
                    try {
                        return mapper.writeValueAsString(route);
                    } catch (JsonProcessingException e) {
                        throw new RuntimeException(e);
                    }
                })
                .collect(Collectors.joining());
    }

    private static HTTPRouteResource.Spec.Match.Path convertSpringPathToHttpRoutePath(String springPath) {
        HTTPRouteResource.Spec.Match.Path path = new HTTPRouteResource.Spec.Match.Path();

        if (springPath == null || springPath.isEmpty()) {
            path.type = "PathPrefix";
            path.value = "/";
            return path;
        }

        // If path contains Spring placeholders {variable}, use regex
        if (springPath.contains("{")) {
            path.type = "RegularExpression";
            path.value = springPath.replaceAll("\\{([^/]+?)}", "([^/]+)");
        } else {
            path.type = "PathPrefix";
            path.value = springPath;
        }
        if (!path.value.startsWith("/")) {
            path.value = "/" + path.value;
        }
        return path;
    }

    private static String getGatewayName(RouteType routeType) {
        return switch (routeType) {
            case FACADE -> FACADE_GATEWAY_NAME;
            case INTERNAL -> INTERNAL_GATEWAY_NAME;
            case PRIVATE -> PRIVATE_GATEWAY_NAME;
            case PUBLIC -> PUBLIC_GATEWAY_NAME;
        };
    }
}

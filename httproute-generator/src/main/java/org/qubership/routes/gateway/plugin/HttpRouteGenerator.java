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
    public record HTTPRouteResource(
            String apiVersion,
            String kind,
            Metadata metadata,
            Spec spec
    ) {
        public record Metadata(String name) {
        }

        public record Spec(
                Set<ParentRef> parentRefs,
                List<Rule> rules
        ) {
            public record ParentRef(String name) {
            }

            public record Rule(List<Match> matches) {
            }

            public record Match(Path path, List<ForwardTo> forwardTo) {
                public record Path(String type, String value) {
                }

                public record ForwardTo(TargetRef targetRef) {
                    public record TargetRef(String kind, String name, int port) {
                    }
                }
            }
        }
    }

    private static List<HTTPRouteResource> createHttpRoutes(String serviceName, Set<HttpRoute> httpRoutes) {
        Map<HttpRoute.Type, List<HttpRoute>> routesByType = httpRoutes.stream()
                .collect(Collectors.groupingBy(HttpRoute::type));

        return routesByType.entrySet().stream().map(entry -> {
            HttpRoute.Type type = entry.getKey();
            List<HttpRoute> routes = entry.getValue();

            HTTPRouteResource.Metadata metadata =
                    new HTTPRouteResource.Metadata(serviceName + "-" + type.name().toLowerCase());

            HTTPRouteResource.Spec.ParentRef parentRef =
                    new HTTPRouteResource.Spec.ParentRef(getGatewayName(type));

            List<HTTPRouteResource.Spec.Rule> ruleList = new ArrayList<>();
            List<HTTPRouteResource.Spec.Match> matchList = new ArrayList<>();

            for (HttpRoute httpRoute : routes) {
                HTTPRouteResource.Spec.Match.Path path =
                        convertSpringPathToHttpRoutePath(httpRoute.path());

                HTTPRouteResource.Spec.Match.ForwardTo.TargetRef targetRef =
                        new HTTPRouteResource.Spec.Match.ForwardTo.TargetRef(
                                "Service",
                                serviceName,
                                BACKEND_SERVICE_PORT
                        );

                HTTPRouteResource.Spec.Match.ForwardTo forwardTo =
                        new HTTPRouteResource.Spec.Match.ForwardTo(targetRef);

                HTTPRouteResource.Spec.Match match =
                        new HTTPRouteResource.Spec.Match(path, List.of(forwardTo));

                matchList.add(match);
            }

            ruleList.add(new HTTPRouteResource.Spec.Rule(matchList));

            HTTPRouteResource.Spec spec =
                    new HTTPRouteResource.Spec(
                            new HashSet<>(Set.of(parentRef)),
                            ruleList
                    );

            return new HTTPRouteResource(
                    "gateway.networking.k8s.io/v1",
                    "HTTPRoute",
                    metadata,
                    spec
            );

        }).toList();
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
        if (!path.startsWith("/")) {
            return "/" + path;
        }
        return path;
    }

    private static String getGatewayName(HttpRoute.Type routeType) {
        return switch (routeType) {
            case FACADE -> FACADE_GATEWAY_NAME;
            case INTERNAL -> INTERNAL_GATEWAY_NAME;
            case PRIVATE -> PRIVATE_GATEWAY_NAME;
            case PUBLIC -> PUBLIC_GATEWAY_NAME;
        };
    }
}

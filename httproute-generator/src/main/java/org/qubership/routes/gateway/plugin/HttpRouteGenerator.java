package org.qubership.routes.gateway.plugin;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
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

            public record Rule(List<Match> matches, List<BackendRef> backendRefs) {
            }

            public record Match(Path path) {
                public record Path(String type, String value) {
                }
            }

            public record BackendRef(String name, Integer port) {
            }
        }
    }

    private static List<HTTPRouteResource> createHttpRoutes(String serviceName, int servicePort, Set<HttpRoute> httpRoutes) {
        Map<HttpRoute.Type, List<HttpRoute>> routesByType = httpRoutes.stream().collect(Collectors.groupingBy(HttpRoute::type));

        return routesByType.entrySet().stream().map(entry -> {
            HttpRoute.Type type = entry.getKey();
            List<HttpRoute> routes = entry.getValue();

            HTTPRouteResource.Metadata metadata = new HTTPRouteResource.Metadata(serviceName + "-" + type.name().toLowerCase());

            HTTPRouteResource.Spec.ParentRef parentRef = new HTTPRouteResource.Spec.ParentRef(type.gatewayName());

            List<HTTPRouteResource.Spec.Rule> ruleList = new ArrayList<>();
            List<HTTPRouteResource.Spec.Match> matches = new ArrayList<>();

            for (HttpRoute route : routes) {
                var path = convertSpringPathToHttpRoutePath(route.path());
                matches.add(new HTTPRouteResource.Spec.Match(path));
            }

            List<HTTPRouteResource.Spec.BackendRef> backendRefs = List.of(new HTTPRouteResource.Spec.BackendRef(serviceName, servicePort));

            ruleList.add(new HTTPRouteResource.Spec.Rule(matches, backendRefs));

            HTTPRouteResource.Spec spec = new HTTPRouteResource.Spec(Set.of(parentRef), ruleList);

            return new HTTPRouteResource("gateway.networking.k8s.io/v1", "HTTPRoute", metadata, spec);
        }).toList();
    }

    public static String generateHttpRoutesYaml(String serviceName, int servicePort, Set<HttpRoute> httpRoutes) {
        List<HTTPRouteResource> routes = createHttpRoutes(serviceName, servicePort, httpRoutes);

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
        if (!path.startsWith("/")) {
            return "/" + path;
        }
        return path;
    }
}

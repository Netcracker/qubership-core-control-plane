package org.qubership.remesh.handler;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import lombok.extern.slf4j.Slf4j;
import org.qubership.remesh.dto.*;
import org.qubership.remesh.dto.gatewayapi.HttpRoute;
import org.qubership.remesh.util.EndpointDTO;
import org.qubership.remesh.util.EndpointParser;
import org.qubership.remesh.util.ObjectMapperProvider;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.regex.Pattern;

@Slf4j
public class VirtualServiceHandler implements CrHandler{
    @Override
    public String getKind() {
        return "VirtualService";
    }

    @Override
    public List<Resource> handle(JsonNode node) {
        try {
            List<Resource> result = new ArrayList<>();

            VirtualServiceYaml original = ObjectMapperProvider.getMapper().treeToValue(node, VirtualServiceYaml.class);

            if (original.getSpec() != null) {
                String gateway = original.getMetadata().getGateway();
                if (gateway == null || gateway.isEmpty()) {
                    throw new IllegalStateException("Required parameter 'gateway' is absent in 'VirtualService' metadata");
                }
                HttpRoute httpRoute = new HttpRoute();
                httpRoute.setMetadata(metadataToHttpRouteMetadata(original.getMetadata()));
                httpRoute.setSpec(virtualServiceToHttpRouteSpec(List.of(gateway), original.getSpec()));
                result.add(httpRoute);
            }
            return result;
        }
        catch (IllegalArgumentException | JsonProcessingException e) {
            log.error("Cannot deserialize RouteConfiguration", e);
            return Collections.emptyList();
        }
    }


    protected HttpRoute.Metadata metadataToHttpRouteMetadata(Metadata metadata) {
        if (metadata == null) {
            return null;
        }

        HttpRoute.Metadata result = new HttpRoute.Metadata();
        result.setName(safeName(metadata));
        result.setNamespace(metadata.getNamespace());
        result.setLabels(metadata.getLabels());
        return result;
    }

    private String safeName(Metadata metadata) {
        if (metadata != null && metadata.getName() != null && !metadata.getName().isEmpty()) {
            return metadata.getName() + "-http-route";
        }
        return "generated-http-route";
    }

    protected HttpRoute.HttpRouteSpec virtualServiceToHttpRouteSpec(List<String> gateways, VirtualService virtualService) {
        HttpRoute.HttpRouteSpec result = new HttpRoute.HttpRouteSpec();
        result.setParentRefs(gatewaysToParentReferences(gateways));
        result.setHostnames(hostsToHostnames(virtualService));
        result.setRules(routeRulesToRules(virtualService));
        return result;
    }

    private List<String> hostsToHostnames(VirtualService virtualService) {
        List<String> hosts = virtualService.getHosts();
        if (hosts != null) {
            if (hosts.size() == 1 && hosts.getFirst().equals("*")) {
                return null;
            }
            else {
                return hosts;
            }
        } else {
            return null;
        }
    }

    // rules — flatten RouteConfig.Routes[*].Rules[*]
    private List<HttpRoute.Rule> routeRulesToRules(VirtualService virtualService) {
        List<HttpRoute.Rule> result = new ArrayList<>();
        RouteConfig routeConfig = virtualService.getRouteConfiguration();
        if (routeConfig != null && routeConfig.getRoutes() != null) {
            for (RouteV3 routeV3 : routeConfig.getRoutes()) {
                for (Rule rule : routeV3.getRules()) {
//                    if (rule.getDeny() != null && Boolean.TRUE.equals(rule.getDeny())) {
//                        continue;
//                    }
                    if (rule.getAllowed() != null && Boolean.FALSE.equals(rule.getAllowed())) {
                        log.debug("    Rule is not allowed - skip");
                        continue;
                    }

                    HttpRoute.Rule newRule = new HttpRoute.Rule();

                    // match
                    List<HttpRoute.Match> matches = new ArrayList<>();
                    HttpRoute.Match match = toMatch(rule.getMatch());
                    if (match != null) {
                        matches.add(match);
                    }
                    newRule.setMatches(matches);

                    // filters: URLRewrite + headers
                    List<HttpRoute.Filter> filters = getFilters(rule, virtualService);
                    newRule.setFilters(filters);

                    // backendRefs from RouteDestination
                    List<HttpRoute.BackendRef> backendRefs = new ArrayList<>();
                    HttpRoute.BackendRef backendRef = toBackendRef(routeV3.getDestination());
                    if (backendRef != null) {
                        backendRefs.add(backendRef);
                    }
                    newRule.setBackendRefs(backendRefs);

                    result.add(newRule);
                }
            }
        }
        return result;
    }

    List<HttpRoute.Filter> getFilters(Rule rule, VirtualService virtualService) {
        List<HttpRoute.Filter> filters = new ArrayList<>();

        filters.addAll(calculateGeneralFilters(rule));

        filters.addAll(calculateHeaderFilter(rule, virtualService));
        return filters;

        // TODO: vs.RateLimit, rule.RateLimit → extensionRef / EnvoyFilter
        // TODO: Timeout / IdleTimeout → implementation-specific policy
        // TODO: LuaFilter → EnvoyFilter
        // TODO: StatefulSession → DestinationRule (sticky sessions)
    }

    List<HttpRoute.Filter> calculateGeneralFilters(Rule rule) {
        List<HttpRoute.Filter> result = new ArrayList<>();
        if (rule.getPrefixRewrite() != null) {
            HttpRoute.Filter filter = new HttpRoute.Filter();
            filter.setType(HttpRoute.FilterType.URLRewrite);
            HttpRoute.URLRewrite urlRewrite = new HttpRoute.URLRewrite();
            HttpRoute.PathRewrite pathRewrite = new HttpRoute.PathRewrite();
            pathRewrite.setType(HttpRoute.PathRewriteType.ReplacePrefixMatch);
            pathRewrite.setReplacePrefixMatch(rule.getPrefixRewrite());
            urlRewrite.setPath(pathRewrite);
            if (rule.getHostRewrite() != null) {
                urlRewrite.setHostname(rule.getHostRewrite());
            }
            filter.setUrlRewrite(urlRewrite);
            result.add(filter);
        } else if (rule.getHostRewrite() != null) {
            HttpRoute.Filter filter = new HttpRoute.Filter();
            filter.setType(HttpRoute.FilterType.URLRewrite);
            HttpRoute.URLRewrite urlRewrite = new HttpRoute.URLRewrite();
            urlRewrite.setHostname(rule.getHostRewrite());
            filter.setUrlRewrite(urlRewrite);
            result.add(filter);
        }
        return result;
    }

    List<HttpRoute.Filter> calculateHeaderFilter(Rule rule, VirtualService virtualService) {
        List<HttpRoute.Filter> result = new ArrayList<>();

        List<HeaderDefinition> addHeaders = new ArrayList<>();
        if (virtualService != null && virtualService.getAddHeaders() != null) {
            addHeaders.addAll(virtualService.getAddHeaders());
        }
        if (rule.getAddHeaders() != null) {
            addHeaders.addAll(rule.getAddHeaders());
        }

        List<String> removeHeaders = new ArrayList<>();
        if (virtualService != null && virtualService.getRemoveHeaders() != null) {
            removeHeaders.addAll(virtualService.getRemoveHeaders());
        }
        if (rule.getRemoveHeaders() != null) {
            removeHeaders.addAll(rule.getRemoveHeaders());
        }

        if (!addHeaders.isEmpty() || !removeHeaders.isEmpty()) {
            HttpRoute.Filter filter = new HttpRoute.Filter();
            filter.setType(HttpRoute.FilterType.RequestHeaderModifier);
            HttpRoute.RequestHeaderModifier requestHeaderModifier = getRequestHeaderModifier(addHeaders, removeHeaders);
            filter.setRequestHeaderModifier(requestHeaderModifier);
            result.add(filter);
        }
        return result;
    }

    HttpRoute.RequestHeaderModifier getRequestHeaderModifier(List<HeaderDefinition> addHeaders, List<String> removeHeaders) {
        HttpRoute.RequestHeaderModifier requestHeaderModifier = new HttpRoute.RequestHeaderModifier();

        List<HttpRoute.Header> add = new ArrayList<>();
        for (HeaderDefinition headerDefinition : addHeaders) {
            HttpRoute.Header header = new HttpRoute.Header();
            header.setName(headerDefinition.getName());
            header.setValue(headerDefinition.getValue());
            add.add(header);
        }

        requestHeaderModifier.setAdd(add);
        requestHeaderModifier.setRemove(removeHeaders);

        return requestHeaderModifier;
    }

    List<HttpRoute.ParentReference> gatewaysToParentReferences(List<String> gateways) {
        List<HttpRoute.ParentReference> parentRefs = new ArrayList<>();
        if (gateways != null) {
            for (String gateway : gateways) {
                HttpRoute.ParentReference parentReference = new HttpRoute.ParentReference();
                parentReference.setGroup("gateway.networking.k8s.io");
                parentReference.setKind("Gateway");
                parentReference.setName(gateway);
                parentRefs.add(parentReference);
            }
        }
        return parentRefs;
    }

    HttpRoute.Match toMatch(RouteMatch match) {
        if (match == null) {
            return null;
        }
        HttpRoute.Match result = new HttpRoute.Match();

        // path
        if (match.getPrefix() != null) {
            HttpRoute.PathMatch pathMatch = new HttpRoute.PathMatch();
            pathMatch.setType(HttpRoute.PathMatchType.PathPrefix);
            pathMatch.setValue(match.getPrefix());
            result.setPath(pathMatch);
        } else if (match.getPath() != null) {
            HttpRoute.PathMatch pathMatch = new HttpRoute.PathMatch();
            pathMatch.setType(HttpRoute.PathMatchType.Exact);
            pathMatch.setValue(match.getPath());
            result.setPath(pathMatch);
        } else if (match.getRegExp() != null) {
            HttpRoute.PathMatch pathMatch = new HttpRoute.PathMatch();
            pathMatch.setType(HttpRoute.PathMatchType.RegularExpression);
            pathMatch.setValue(match.getRegExp());
            result.setPath(pathMatch);
        }

        // headers
        if (match.getHeaderMatchers() != null) {
            List<HttpRoute.HeaderMatch> headers = getHeaderMatches(match.getHeaderMatchers());
            result.setHeaders(headers);
        }

        if (result.getPath() == null && (result.getHeaders() == null || result.getHeaders().isEmpty())) {
            return null;
        }
        return result;
    }

    List<HttpRoute.HeaderMatch> getHeaderMatches(List<HeaderMatcher> matches) {
        List<HttpRoute.HeaderMatch> headers = new ArrayList<>();
        for (HeaderMatcher headerMatcher : matches) {
            HttpRoute.HeaderMatch headerMatch = new HttpRoute.HeaderMatch();
            headerMatch.setName(headerMatcher.getName());
            if (headerMatcher.getExactMatch() != null) {
                headerMatch.setType(HttpRoute.HeaderMatchType.Exact);
                headerMatch.setValue(headerMatcher.getExactMatch());
            }
            else if (headerMatcher.getSafeRegexMatch() != null) {
                headerMatch.setType(HttpRoute.HeaderMatchType.RegularExpression);
                headerMatch.setValue(headerMatcher.getSafeRegexMatch());
            } else if (headerMatcher.getPrefixMatch() != null) {
                headerMatch.setType(HttpRoute.HeaderMatchType.RegularExpression);
                headerMatch.setValue("^" + Pattern.quote(headerMatcher.getPrefixMatch()) + ".*$");
            }
            else if (headerMatcher.getSuffixMatch() != null) {
                headerMatch.setType(HttpRoute.HeaderMatchType.RegularExpression);
                headerMatch.setValue(".*" + Pattern.quote(headerMatcher.getSuffixMatch()) + "$");
            }
            else if (headerMatcher.isPresentMatch()) {
                headerMatch.setType(HttpRoute.HeaderMatchType.RegularExpression);
                headerMatch.setValue(".*");
            }
            else
            {
                //TODO VLLA the rest types are not supported
                log.warn("Header match {} is unsupported", headerMatch);
            }

            if (headerMatch.getType() != null) {
                headers.add(headerMatch);
            }
        }
        return headers;
    }

    HttpRoute.BackendRef toBackendRef(RouteDestination dst) {
        if (dst == null) {
            return null;
        }
        EndpointDTO endpoint = EndpointParser.parse(dst.getEndpoint());

        HttpRoute.BackendRef backendRef = new HttpRoute.BackendRef();
        backendRef.setKind("Service");
        backendRef.setName(endpoint.host());
        backendRef.setPort(endpoint.port());
        // TODO: dst.TlsSupported / TlsEndpoint / HttpVersion / TlsConfigName → DestinationRule/Policy

        return backendRef;
    }
}

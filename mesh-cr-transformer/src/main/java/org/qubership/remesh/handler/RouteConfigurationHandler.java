package org.qubership.remesh.handler;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import lombok.extern.slf4j.Slf4j;
import org.qubership.remesh.dto.RouteConfigurationYaml;
import org.qubership.remesh.dto.VirtualService;
import org.qubership.remesh.dto.gatewayapi.HttpRoute;
import org.qubership.remesh.util.ObjectMapperProvider;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;

@Slf4j
public class RouteConfigurationHandler extends VirtualServiceHandler {
    @Override
    public String getKind() {
        return "RouteConfiguration";
    }

    @Override
    public List<Resource> handle(JsonNode node) {
        try {
            List<Resource> result = new ArrayList<>();

            RouteConfigurationYaml original = ObjectMapperProvider.getMapper().treeToValue(node, RouteConfigurationYaml.class);

            if (original.getSpec() != null && original.getSpec().getVirtualServices() != null) {
                for (VirtualService virtualService : original.getSpec().getVirtualServices()) {
                    HttpRoute httpRoute = new HttpRoute();
                    httpRoute.setMetadata(metadataToHttpRouteMetadata(original.getMetadata()));
                    httpRoute.setSpec(virtualServiceToHttpRouteSpec(original.getSpec().getGateways(), virtualService));
                    result.add(httpRoute);
                }
            }
            return result;
        } catch (IllegalArgumentException | JsonProcessingException e) {
            log.error("Cannot deserialize RouteConfiguration", e);
            return Collections.emptyList();
        }
    }
}

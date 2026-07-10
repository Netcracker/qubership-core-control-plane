package org.qubership.remesh.handler;

import com.fasterxml.jackson.core.JsonProcessingException;
import lombok.extern.slf4j.Slf4j;
import org.qubership.remesh.dto.*;
import org.qubership.remesh.dto.gatewayapi.HttpRoute;
import org.qubership.remesh.util.ObjectMapperProvider;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.Map;

@Slf4j
public class RouteConfigurationHandler extends VirtualServiceHandler {
    @Override
    public String getKind() {
        return "RouteConfiguration";
    }

    @Override
    public List<Resource> handle(MeshResourceFragment fragment, Map<String, Object> config) {
        try {
            List<Resource> result = new ArrayList<>();

            // Deserialize only the spec part
            RoutingConfigRequestV3 spec = ObjectMapperProvider.getMapper()
                    .treeToValue(fragment.getSpec(), RoutingConfigRequestV3.class);

            if (spec != null && spec.getVirtualServices() != null) {
                for (VirtualService virtualService : spec.getVirtualServices()) {
                    HttpRoute httpRoute = new HttpRoute();
                    httpRoute.setSpec(virtualServiceToHttpRouteSpec(spec.getGateways(), virtualService, config));
                    httpRoute.setRawMetadata(fragment.getRawMetadata());
                    result.add(httpRoute);
                }
            }
            return result;
        }
        catch (IllegalArgumentException | JsonProcessingException e) {
            log.error("Cannot deserialize RouteConfiguration", e);
            return Collections.emptyList();
        }
    }
}

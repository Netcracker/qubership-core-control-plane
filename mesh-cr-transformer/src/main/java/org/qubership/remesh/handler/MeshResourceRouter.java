package org.qubership.remesh.handler;

import com.fasterxml.jackson.databind.JsonNode;
import lombok.extern.slf4j.Slf4j;

import java.util.Collections;
import java.util.List;
import java.util.function.Function;

@Slf4j
public class MeshResourceRouter {
    private static final String CORE_NETCRACKER_COM_API_VERSION = "core.netcracker.com/v1";
    private static final String MESH_KIND = "Mesh";

    private final Function<String, CrHandler> handlerProvider;

    public MeshResourceRouter() {
        this(CrHandlerRegistry::get);
    }

    public MeshResourceRouter(Function<String, CrHandler> handlerProvider) {
        this.handlerProvider = handlerProvider;
    }

    public List<Resource> route(JsonNode node) {
        if (node == null) {
            return List.of();
        }

        JsonNode apiVersion = node.get("apiVersion");
        JsonNode kind = node.get("kind");
        JsonNode subKind = node.get("subKind");

        if (apiVersion == null || kind == null || subKind == null) {
            return List.of();
        }

        if (!isMeshResource(apiVersion, kind)) {
            return List.of();
        }

        log.debug("    Start processing Mesh resource with subKind {}", subKind.asText());

        CrHandler handler = handlerProvider.apply(subKind.asText());
        if (handler == null) {
            log.warn("    Handler not found for kind {}", subKind.asText());
            return List.of();
        }

        List<Resource> resources = handler.handle(node);
        if (resources == null) {
            return Collections.emptyList();
        }

        return resources;
    }

    boolean isMeshResource(JsonNode apiVersion, JsonNode kind) {
        return CORE_NETCRACKER_COM_API_VERSION.equals(apiVersion.asText()) && MESH_KIND.equals(kind.asText());
    }
}

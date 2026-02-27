package org.qubership.remesh.handler;

import lombok.NonNull;
import lombok.extern.slf4j.Slf4j;

import java.util.Collections;
import java.util.List;
import java.util.Map;
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

    public List<Resource> route(MeshResourceFragment fragment) {
        return route(fragment, Collections.emptyMap());
    }

    public List<Resource> route(MeshResourceFragment fragment, @NonNull Map<String, Object> config) {
        if (fragment == null) {
            return List.of();
        }

        if (!isMeshResource(fragment.getApiVersion(), fragment.getKind())) {
            return List.of();
        }

        CrHandler handler = handlerProvider.apply(fragment.getSubKind());
        if (handler == null) {
            log.warn("       Handler not found for kind {}. Manual migration is needed", fragment.getSubKind());
            return List.of();
        }

        log.info("------ Handle mesh fragment[{}] '{}'", fragment.getIndex(), fragment.getFullKind());

        List<Resource> resources = handler.handle(fragment, config);
        if (resources == null) {
            return Collections.emptyList();
        }

        return resources;
    }

    boolean isMeshResource(String apiVersion, String kind) {
        return CORE_NETCRACKER_COM_API_VERSION.equals(apiVersion) && MESH_KIND.equals(kind);
    }
}

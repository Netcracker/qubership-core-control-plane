package org.qubership.remesh.handler;

import java.util.List;
import java.util.Map;

public interface CrHandler {
    String getKind();

    /**
     * Handles a parsed Mesh resource fragment and produces Gateway API resources.
     *
     * @param fragment the parsed and validated Mesh resource fragment
     * @param config   optional config map (e.g. from --config YAML); may be empty, never null
     * @return list of generated resources
     */
    List<Resource> handle(MeshResourceFragment fragment, Map<String, Object> config);
}

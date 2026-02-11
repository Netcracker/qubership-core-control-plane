package org.qubership.remesh.handler;

import java.util.List;

public interface CrHandler {
    String getKind();

    /**
     * Handles a parsed Mesh resource fragment and produces Gateway API resources.
     *
     * @param fragment the parsed and validated Mesh resource fragment
     * @return list of generated resources
     */
    List<Resource> handle(MeshResourceFragment fragment);
}

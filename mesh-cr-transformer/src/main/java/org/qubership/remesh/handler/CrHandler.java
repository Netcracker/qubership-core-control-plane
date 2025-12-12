package org.qubership.remesh.handler;

import com.fasterxml.jackson.databind.JsonNode;

import java.util.List;

public interface CrHandler {
    String getKind();
    List<Resource> handle(JsonNode node);
}

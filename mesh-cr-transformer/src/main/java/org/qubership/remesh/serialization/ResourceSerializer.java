package org.qubership.remesh.serialization;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.qubership.remesh.handler.Resource;

import java.io.IOException;

/**
 * Serializer for Gateway API resources.
 * Uses raw metadata when available to preserve Helm templates, comments, and original formatting.
 */
public class ResourceSerializer {
    private final ObjectMapper mapper;

    public ResourceSerializer(ObjectMapper mapper) {
        this.mapper = mapper;
    }

    /**
     * Serializes a resource to YAML, using raw metadata if available.
     * This preserves Helm templates, comments, and original formatting in metadata section.
     *
     * @param resource the resource to serialize
     * @return YAML string representation
     * @throws IOException if serialization fails
     */
    public String serialize(Resource resource) throws IOException {
        String rawMetadata = resource.getRawMetadata();

        if (rawMetadata == null || rawMetadata.isEmpty()) {
            // Fallback to standard serialization
            return mapper.writeValueAsString(resource);
        }

        return serializeWithRawMetadata(resource, rawMetadata);
    }

    private String serializeWithRawMetadata(Resource resource, String rawMetadata) throws IOException {
        StringBuilder sb = new StringBuilder();
        sb.append("---\n");
        sb.append("apiVersion: ").append(resource.getApiVersion()).append("\n");
        sb.append("kind: ").append(resource.getKind()).append("\n");
        sb.append(rawMetadata).append("\n");

        // Serialize only the spec part
        JsonNode fullNode = mapper.valueToTree(resource);
        JsonNode specNode = fullNode.get("spec");

        if (specNode != null) {
            String specYaml = mapper.writeValueAsString(specNode);
            appendSpec(sb, specYaml);
        }

        return sb.toString();
    }

    private void appendSpec(StringBuilder sb, String specYaml) {
        // Remove leading "---\n" if present
        if (specYaml.startsWith("---")) {
            specYaml = specYaml.substring(specYaml.indexOf('\n') + 1);
        }

        sb.append("spec:\n");

        // Indent spec content
        String[] specLines = specYaml.split("\n");
        for (String line : specLines) {
            if (!line.isEmpty()) {
                sb.append("  ").append(line).append("\n");
            }
        }
    }
}

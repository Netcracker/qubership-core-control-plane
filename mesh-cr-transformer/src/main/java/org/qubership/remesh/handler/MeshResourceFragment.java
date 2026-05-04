package org.qubership.remesh.handler;

import com.fasterxml.jackson.databind.JsonNode;
import lombok.Getter;

/**
 * DTO representing a parsed Mesh resource fragment.
 * Contains both raw metadata (preserving templates/comments)
 * and parsed fields (apiVersion, kind, subKind, spec, metadata).
 * <p>
 * This class is immutable and validated upon creation.
 */
@Getter
public class MeshResourceFragment {
    private final int index;
    private final String rawMetadata;
    private final String apiVersion;
    private final String kind;
    private final String subKind;
    private final JsonNode metadata;
    private final JsonNode spec;

    private MeshResourceFragment(int index, String rawMetadata, String apiVersion, String kind, String subKind,
                                  JsonNode metadata, JsonNode spec) {
        this.index = index;
        this.rawMetadata = rawMetadata;
        this.apiVersion = apiVersion;
        this.kind = kind;
        this.subKind = subKind;
        this.metadata = metadata;
        this.spec = spec;
    }

    /**
     * Creates a MeshResourceFragment from a parsed JsonNode and raw metadata.
     * Performs validation to ensure all required fields are present.
     *
     * @param index       the index of the fragment in the source file (0-based)
     * @param node        the parsed JSON/YAML node
     * @param rawMetadata the raw metadata section from original YAML
     * @return a validated MeshResourceFragment
     * @throws IllegalArgumentException if structure is invalid (missing required fields)
     */
    public static MeshResourceFragment create(int index, JsonNode node, String rawMetadata) {
        if (node == null) {
            throw new IllegalArgumentException("Node cannot be null");
        }

        JsonNode apiVersionNode = node.get("apiVersion");
        JsonNode kindNode = node.get("kind");
        JsonNode subKindNode = node.get("subKind");
        JsonNode metadataNode = node.get("metadata");
        JsonNode specNode = node.get("spec");

        if (apiVersionNode == null || !apiVersionNode.isTextual()) {
            throw new IllegalArgumentException("Missing or invalid 'apiVersion' field");
        }
        if (kindNode == null || !kindNode.isTextual()) {
            throw new IllegalArgumentException("Missing or invalid 'kind' field");
        }
        if (subKindNode == null || !subKindNode.isTextual()) {
            throw new IllegalArgumentException("Missing or invalid 'subKind' field");
        }
        if (specNode == null) {
            throw new IllegalArgumentException("Missing 'spec' field");
        }

        return new MeshResourceFragment(
                index,
                rawMetadata,
                apiVersionNode.asText(),
                kindNode.asText(),
                subKindNode.asText(),
                metadataNode,
                specNode
        );
    }

    public String getFullKind() {
        return this.getApiVersion() + ":" + this.getKind() + ":" + this.getSubKind();
    }
}

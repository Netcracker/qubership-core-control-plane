package org.qubership.remesh.handler;

import com.fasterxml.jackson.databind.JsonNode;
import lombok.extern.slf4j.Slf4j;
import org.qubership.remesh.serialization.RawYamlExtractor;
import org.qubership.remesh.serialization.YamlPreprocessor;

/**
 * Factory for parsing raw YAML documents into MeshResourceFragment objects.
 * Encapsulates all parsing logic: metadata extraction, YAML preprocessing,
 * and structure validation.
 */
@Slf4j
public class MeshResourceFragmentParser {
    private final YamlPreprocessor yamlPreprocessor;
    private final RawYamlExtractor rawYamlExtractor;

    public MeshResourceFragmentParser(
            YamlPreprocessor yamlPreprocessor,
            RawYamlExtractor rawYamlExtractor
    ) {
        this.yamlPreprocessor = yamlPreprocessor;
        this.rawYamlExtractor = rawYamlExtractor;
    }

    /**
     * Parses a raw YAML document into a MeshResourceFragment.
     * Returns null if the document is empty or cannot be parsed.
     *
     * @param rawDoc raw YAML document string
     * @param index  the index of the fragment in the source file (0-based)
     * @return parsed fragment or null if document is empty/invalid
     */
    public MeshResourceFragment parse(String rawDoc, int index) {
        if (rawDoc == null || rawDoc.isBlank()) {
            return null;
        }

        try {
            String rawMetadata = rawYamlExtractor.extractMetadata(rawDoc);

            JsonNode node = yamlPreprocessor.readAsJsonNode(rawDoc);
            if (node == null) {
                return null;
            }

            return MeshResourceFragment.create(index, node, rawMetadata);
        } catch (IllegalArgumentException e) {
            log.debug("Failed to parse fragment: {}", e.getMessage());
            return null;
        }
    }
}

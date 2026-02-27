package org.qubership.remesh.serialization;

import com.fasterxml.jackson.databind.JsonNode;
import org.junit.jupiter.api.Test;
import org.qubership.remesh.util.ObjectMapperProvider;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;

class YamlPreprocessorTest {

    @Test
    void quotesInlineTemplateValues() {
        YamlPreprocessor preprocessor = new YamlPreprocessor(ObjectMapperProvider.getMapper());

        String yaml = "key: {{ value }}";

        String processed = preprocessor.quoteInlineTemplates(yaml);

        assertEquals("key: '{{ value }}'", processed.trim());
    }

    @Test
    void replacesStandaloneTemplates() {
        YamlPreprocessor preprocessor = new YamlPreprocessor(ObjectMapperProvider.getMapper());

        String yaml = "{{ something }}";

        String processed = preprocessor.replaceStandaloneTemplates(yaml);

        assertEquals("# {{ something }}", processed.trim());
    }

    @Test
    void readsJsonNodeAfterPreprocessing() {
        YamlPreprocessor preprocessor = new YamlPreprocessor(ObjectMapperProvider.getMapper());

        JsonNode node = preprocessor.readAsJsonNode("apiVersion: core.netcracker.com/v1\nkind: Mesh\nsubKind: Demo");

        assertNotNull(node);
        assertEquals("Mesh", node.get("kind").asText());
    }
}

package org.qubership.remesh.validation;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

class JsonSchemaValidatorTest {

    private final ObjectMapper mapper = new ObjectMapper();

    @Test
    void validatesResourceAgainstSchema() {
        JsonNode resource = mapper.createObjectNode()
                .put("apiVersion", "example.com/v1")
                .put("kind", "Demo")
                .set("spec", mapper.createObjectNode().put("replicas", 2));

        assertDoesNotThrow(() -> JsonSchemaValidator.validate(resource, "valid-crd.yaml"));
    }

    @Test
    void validationErrorsDoNotThrow() {
        JsonNode resource = mapper.createObjectNode()
                .put("apiVersion", "example.com/v1")
                .put("kind", "Demo");

        assertDoesNotThrow(() -> JsonSchemaValidator.validate(resource, "valid-crd.yaml"));
    }

    @Test
    void failsWhenSchemaNameMissing() {
        JsonNode resource = mapper.createObjectNode();
        assertThrows(IllegalArgumentException.class, () -> JsonSchemaValidator.validate(resource, " "));
    }

    @Test
    void failsWhenSchemaNotFound() {
        assertThrows(IllegalStateException.class, () -> JsonSchemaValidator.loadSchema("missing.yaml"));
    }
}

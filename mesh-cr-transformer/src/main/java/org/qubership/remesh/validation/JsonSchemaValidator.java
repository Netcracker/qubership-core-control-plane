package org.qubership.remesh.validation;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;
import com.networknt.schema.JsonMetaSchema;
import com.networknt.schema.JsonSchema;
import com.networknt.schema.JsonSchemaFactory;
import com.networknt.schema.NonValidationKeyword;
import com.networknt.schema.SpecVersion;
import com.networknt.schema.ValidationMessage;
import lombok.extern.slf4j.Slf4j;

import java.io.InputStream;
import java.util.Arrays;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;

@Slf4j
public final class JsonSchemaValidator {
    private static final ObjectMapper YAML_MAPPER = new ObjectMapper(new YAMLFactory());
    private static final JsonSchemaFactory JSON_SCHEMA_FACTORY = buildSchemaFactory();
    private static final Map<String, JsonSchema> SCHEMA_CACHE = new ConcurrentHashMap<>();
    private static final String SCHEMAS_DIR = "schemas/";

    public static void validate(JsonNode resourceJson, String schemaFileName) {
        if (schemaFileName == null || schemaFileName.isBlank()) {
            throw new IllegalArgumentException("Schema file name must be provided");
        }
        try {
            JsonSchema schema = SCHEMA_CACHE.computeIfAbsent(
                    schemaFileName,
                    JsonSchemaValidator::loadSchema);

            Set<ValidationMessage> errors = schema.validate(resourceJson);

            if (!errors.isEmpty()) {
                StringBuilder sb = new StringBuilder("Resource validation failed:\n");
                for (ValidationMessage e : errors) {
                    sb.append(" - ")
                            .append(e.getPath())
                            .append(": ")
                            .append(e.getMessage())
                            .append('\n');
                }
                log.error(sb.toString());
            }
        } catch (Exception e) {
            throw new IllegalArgumentException(
                    "Failed to validate resource against schema %s".formatted(schemaFileName),
                    e);
        }
    }

    static JsonSchema loadSchema(String schemaFileName) {
        String resourcePath = SCHEMAS_DIR + schemaFileName;
        try (InputStream is =
                     JsonSchemaValidator.class
                             .getClassLoader()
                             .getResourceAsStream(resourcePath)) {

            if (is == null) {
                throw new IllegalStateException("Schema resource not found: " + resourcePath);
            }

            JsonNode crd = YAML_MAPPER.readTree(is);

            JsonNode schemaNode = null;
            for (JsonNode v : crd.at("/spec/versions")) {
                if ("v1".equals(v.path("name").asText())) {
                    schemaNode = v.at("/schema/openAPIV3Schema");
                    break;
                }
            }

            if (schemaNode == null) {
                throw new IllegalStateException("v1 schema not found in CRD: " + resourcePath);
            }

            return JSON_SCHEMA_FACTORY.getSchema(schemaNode);

        } catch (Exception e) {
            throw new IllegalStateException(
                    "Failed to load CRD schema: " + schemaFileName, e);
        }
    }

    static JsonSchemaFactory buildSchemaFactory() {
        JsonSchemaFactory baseFactory =
                JsonSchemaFactory.getInstance(SpecVersion.VersionFlag.V201909);

        JsonMetaSchema baseMetaSchema = JsonMetaSchema.getV201909();

        JsonMetaSchema kubeMetaSchema = JsonMetaSchema
                .builder("https://json-schema.org/draft/2019-09/schema", baseMetaSchema)
                .addKeywords(Arrays.asList(
                        new NonValidationKeyword("x-kubernetes-validations"),
                        new NonValidationKeyword("x-kubernetes-list-type"),
                        new NonValidationKeyword("x-kubernetes-map-type"),
                        new NonValidationKeyword("x-kubernetes-preserve-unknown-fields")
                ))
                .build();

        return JsonSchemaFactory
                .builder(baseFactory)
                .addMetaSchema(kubeMetaSchema)
                .build();
    }

    private JsonSchemaValidator() {}
}

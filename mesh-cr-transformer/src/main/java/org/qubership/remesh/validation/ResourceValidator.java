package org.qubership.remesh.validation;

import com.fasterxml.jackson.databind.JsonNode;
import lombok.extern.slf4j.Slf4j;
import org.qubership.remesh.handler.Resource;
import org.qubership.remesh.util.ObjectMapperProvider;

import java.util.Optional;
import java.util.regex.Pattern;

import static org.qubership.remesh.validation.JsonSchemaValidator.validate;

@Slf4j
public class ResourceValidator {
    private static final Pattern API_VERSION_SUFFIX = Pattern.compile("/v\\w+$");

    public void validateResource(Resource resource) {
        JsonNode resourceNode = ObjectMapperProvider.getMapper().valueToTree(resource);
        log.info("    Start validating {}", Optional.ofNullable(resource.getKind()).orElse("resource"));
        String schemaFileName = schemaFileName(resource);
        validate(resourceNode, schemaFileName);
    }

    public String schemaFileName(Resource resource) {
        String apiVersion = Optional.ofNullable(resource.getApiVersion()).orElse("");
        String kind = Optional.ofNullable(resource.getKind()).orElse("");

        String baseApiVersion = API_VERSION_SUFFIX.matcher(apiVersion).replaceFirst("");

        return (baseApiVersion + "_" + kind).toLowerCase() + ".yaml";
    }
}

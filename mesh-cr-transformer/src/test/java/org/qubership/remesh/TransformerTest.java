package org.qubership.remesh;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.qubership.remesh.handler.Resource;
import org.qubership.remesh.handler.MeshResourceRouter;
import org.qubership.remesh.serialization.YamlPreprocessor;
import org.qubership.remesh.util.ObjectMapperProvider;
import org.qubership.remesh.validation.ResourceValidator;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

class TransformerTest {

    @Test
    void writesHandledResourcesToNewFile() throws IOException {
        Path dir = Files.createTempDirectory("remesh-test");
        Path input = dir.resolve("resource.yaml");
        Files.writeString(input, "apiVersion: demo/v1\nkind: Mesh\nsubKind: Demo");

        ObjectMapper mapper = ObjectMapperProvider.getMapper();

        YamlPreprocessor preprocessor = new YamlPreprocessor(mapper) {
            @Override
            public JsonNode readAsJsonNode(String rawDoc) {
                try {
                    return mapper.readTree("apiVersion: core.netcracker.com/v1\nkind: Mesh\nsubKind: Demo");
                } catch (Exception e) {
                    return null;
                }
            }
        };

        RecordingRouter router = new RecordingRouter();
        NoopValidator validator = new NoopValidator();

        TransformerService transformerService = new TransformerService(preprocessor, router, validator, mapper);

        transformerService.transform(dir, true);

        Path oldFile = dir.resolve("resource.yaml_old");
        Path output = dir.resolve("resource.yaml");
        assertTrue(Files.exists(oldFile));
        assertTrue(Files.exists(output));
        String content = Files.readString(output);
        assertTrue(content.contains("DemoResource"));
        assertEquals(1, router.handledDocuments);
        assertEquals(1, validator.validatedResources);
    }

    private static class RecordingRouter extends MeshResourceRouter {
        private int handledDocuments = 0;

        @Override
        public List<Resource> route(JsonNode node) {
            handledDocuments++;
            List<Resource> resources = new ArrayList<>();
            resources.add(new DemoResource());
            return resources;
        }
    }

    private static class NoopValidator extends ResourceValidator {
        private int validatedResources = 0;

        @Override
        public void validateResource(Resource resource) {
            validatedResources++;
        }
    }

    private static class DemoResource implements Resource {
        @Override
        public String getApiVersion() {
            return "demo/v1";
        }

        @Override
        public String getKind() {
            return "DemoResource";
        }
    }
}

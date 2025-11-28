package org.qubership.remesh;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.extern.slf4j.Slf4j;
import org.qubership.remesh.handler.Resource;
import org.qubership.remesh.handler.MeshResourceRouter;
import org.qubership.remesh.serialization.YamlPreprocessor;
import org.qubership.remesh.util.ObjectMapperProvider;
import org.qubership.remesh.validation.ResourceValidator;

import java.io.IOException;
import java.io.Writer;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.stream.Stream;

@Slf4j
public class TransformerService {
    public static final String FRAGMENT_DELIMITER = "(?m)^---\\s*$";
    private final YamlPreprocessor yamlPreprocessor;
    private final MeshResourceRouter meshResourceRouter;
    private final ResourceValidator resourceValidator;
    private final ObjectMapper mapper;

    public TransformerService() {
        this(new YamlPreprocessor(ObjectMapperProvider.getMapper()),
                new MeshResourceRouter(),
                new ResourceValidator(),
                ObjectMapperProvider.getMapper()
        );
    }

    public TransformerService(YamlPreprocessor yamlPreprocessor,
                              MeshResourceRouter meshResourceRouter,
                              ResourceValidator resourceValidator,
                              ObjectMapper mapper) {
        this.yamlPreprocessor = yamlPreprocessor;
        this.meshResourceRouter = meshResourceRouter;
        this.resourceValidator = resourceValidator;
        this.mapper = mapper;
    }

    public void transform(Path dir, boolean validate) throws IOException {
        log.info("Start transforming in dir '{}'", dir);
        try (Stream<Path> stream = Files.walk(dir)) {
            stream.filter(Files::isRegularFile)
                    .filter(this::isYaml)
                    .forEach(file -> processFile(file, validate));
        }
    }

    boolean isYaml(Path p) {
        String name = p.getFileName().toString().toLowerCase();
        return name.endsWith(".yaml") || name.endsWith(".yml");
    }

    void processFile(Path file, boolean validate) {
        log.info("=== Processing file '{}' ===", file);
        Path oldFile = file.resolveSibling(file.getFileName().toString() + "_old");

        try {
            Files.move(file, oldFile);
        } catch (IOException e) {
            log.error("Failed to rename original file '{}'", file, e);
            return;
        }

        try (Writer writer = Files.newBufferedWriter(file)) {
            String content = Files.readString(oldFile, StandardCharsets.UTF_8);
            String[] documents = content.split(FRAGMENT_DELIMITER);

            int fragmentIndex = 0;
            for (String rawDoc : documents) {
                if (rawDoc == null || rawDoc.isBlank()) {
                    continue;
                }

                fragmentIndex++;
                log.info("--- Start processing fragment {} in file '{}'", fragmentIndex, file);
                try {
                    JsonNode node = yamlPreprocessor.readAsJsonNode(rawDoc);
                    if (node == null) {
                        continue;
                    }

                    List<Resource> resources = meshResourceRouter.route(node);
                    if (resources == null || resources.isEmpty()) {
                        continue;
                    }

                    for (Resource resource : resources) {
                        if (validate) {
                            resourceValidator.validateResource(resource);
                        }
                        writer.write(mapper.writeValueAsString(resource));
                    }
                } finally {
                    log.info("--- Finished processing fragment {} in file '{}'\n", fragmentIndex, file);
                }
            }

        } catch (IOException e) {
            log.error("Failed to process file '{}'", file, e);
            return;
        }

        log.info("=== Output file is '{}' ===\n", file);
    }
}

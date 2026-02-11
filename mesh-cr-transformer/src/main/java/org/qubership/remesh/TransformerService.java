package org.qubership.remesh;

import lombok.extern.slf4j.Slf4j;
import org.qubership.remesh.handler.MeshResourceFragment;
import org.qubership.remesh.handler.MeshResourceFragmentParser;
import org.qubership.remesh.handler.Resource;
import org.qubership.remesh.handler.MeshResourceRouter;
import org.qubership.remesh.serialization.RawYamlExtractor;
import org.qubership.remesh.serialization.ResourceSerializer;
import org.qubership.remesh.serialization.YamlPreprocessor;
import org.qubership.remesh.util.ObjectMapperProvider;
import org.qubership.remesh.validation.ResourceValidator;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Collectors;
import java.util.stream.Stream;

@Slf4j
public class TransformerService {
    public static final String FRAGMENT_DELIMITER = "(?m)^---\\s*$";
    private final MeshResourceFragmentParser fragmentParser;
    private final MeshResourceRouter meshResourceRouter;
    private final ResourceValidator resourceValidator;
    private final ResourceSerializer serializer;

    public TransformerService() {
        this(new MeshResourceFragmentParser(
                        new YamlPreprocessor(ObjectMapperProvider.getMapper()),
                        new RawYamlExtractor()
                ),
                new MeshResourceRouter(),
                new ResourceValidator(),
                new ResourceSerializer(ObjectMapperProvider.getMapper())
        );
    }

    public TransformerService(MeshResourceFragmentParser fragmentParser,
                              MeshResourceRouter meshResourceRouter,
                              ResourceValidator resourceValidator,
                              ResourceSerializer serializer) {
        this.fragmentParser = fragmentParser;
        this.meshResourceRouter = meshResourceRouter;
        this.resourceValidator = resourceValidator;
        this.serializer = serializer;
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

    String insertSuffixBeforeExtension(String filename, String suffix) {
        int dotIndex = filename.lastIndexOf('.');
        if (dotIndex > 0) {
            return filename.substring(0, dotIndex) + suffix + filename.substring(dotIndex);
        }
        return filename + suffix;
    }

    void processFile(Path file, boolean validate) {
        log.info("=== Processing file '{}' ===", file);

        List<Resource> allResources = new ArrayList<>();

        try {
            String content = Files.readString(file, StandardCharsets.UTF_8);
            String[] documents = content.split(FRAGMENT_DELIMITER);

            for (int i = 0; i < documents.length; i++) {
                String rawDoc = documents[i];
                if (rawDoc == null || rawDoc.isBlank()) {
                    continue;
                }

                MeshResourceFragment fragment = fragmentParser.parse(rawDoc, i);
                if (fragment == null) {
                    continue;
                }

                List<Resource> resources = meshResourceRouter.route(fragment);
                if (resources == null || resources.isEmpty()) {
                    continue;
                }

                for (Resource resource : resources) {
                    if (validate) {
                        resourceValidator.validateResource(resource);
                    }
                    allResources.add(resource);
                }
                String resourcesForLogs = resources.stream()
                        .map(resource -> "'" + resource.getFullVersion() + "'")
                        .collect(Collectors.joining(", "));

                log.info("------ Fragment[{}] '{}' transformed to: {}", fragment.getIndex(), fragment.getFullKind(), resourcesForLogs);
            }

        } catch (IOException e) {
            log.error("Failed to process file '{}'", file, e);
            return;
        }

        if (allResources.isEmpty()) {
            log.debug("=== No resources to transform, skipping output file ===");
            return;
        }

        Path newFile = file.resolveSibling(insertSuffixBeforeExtension(file.getFileName().toString(), "-new"));
        try {
            StringBuilder sb = new StringBuilder();
            for (Resource resource : allResources) {
                sb.append(serializer.serialize(resource));
            }
            Files.writeString(newFile, sb.toString(), StandardCharsets.UTF_8);
        } catch (IOException e) {
            log.error("Failed to write output file '{}'", newFile, e);
            return;
        }

        log.info("=== Output: '{}' ===\n", newFile);
    }
}

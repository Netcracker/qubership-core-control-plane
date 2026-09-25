package org.qubership.remesh;

import com.fasterxml.jackson.core.type.TypeReference;
import lombok.extern.slf4j.Slf4j;
import org.qubership.remesh.util.ObjectMapperProvider;
import picocli.CommandLine;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Collections;
import java.util.Map;
import java.util.concurrent.Callable;

@Slf4j
@CommandLine.Command(name = "transform", description = "Performs automatic migration steps")
public class TransformCli implements Callable<Integer> {

    @SuppressWarnings("unused")
    @CommandLine.Option(names = {"-d", "--dir"}, description = "Dir to process", defaultValue = ".")
    private Path directory;

    @SuppressWarnings("unused")
    @CommandLine.Option(names = {"-v", "--validate"}, description = "Run validation", defaultValue = "false")
    private boolean validationEnabled;

    @SuppressWarnings("unused")
    @CommandLine.Option(names = {"-c", "--config"}, description = "Path to YAML config file (optional)")
    private Path configFile;

    @Override
    public Integer call() throws Exception {
        Path dir = directory != null ? directory : Path.of(".");
        if (!Files.isDirectory(dir)) {
            log.error("Not a directory: {}", dir.toAbsolutePath());
            return 1;
        }

        Map<String, Object> config = loadConfig();
        new TransformerService().transform(dir, validationEnabled, config);

        return 0;
    }

    private Map<String, Object> loadConfig() {
        if (configFile == null || !Files.isRegularFile(configFile)) {
            return Collections.emptyMap();
        }
        try {
            return ObjectMapperProvider.getMapper().readValue(
                    configFile.toFile(),
                    new TypeReference<Map<String, Object>>() {}
            );
        } catch (Exception e) {
            log.error("Failed to load config from '{}'", configFile.toAbsolutePath(), e);
            return Collections.emptyMap();
        }
    }
}
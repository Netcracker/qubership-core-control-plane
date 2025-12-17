package org.qubership.remesh;

import lombok.extern.slf4j.Slf4j;
import picocli.CommandLine;

import java.nio.file.Files;
import java.nio.file.Path;
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

    @Override
    public Integer call() throws Exception {
        Path dir = directory != null ? directory : Path.of(".");
        if (!Files.isDirectory(dir)) {
            log.error("Not a directory: {}", dir.toAbsolutePath());
            return 1;
        }

        new TransformerService().transform(dir, validationEnabled);

        return 0;
    }
}
package org.qubership.remesh;

import lombok.extern.slf4j.Slf4j;
import picocli.CommandLine;

@Slf4j
public class Main {
    public static void main(String[] args) {
        int exitCode = new CommandLine(new TransformCli()).execute(args);
        System.exit(exitCode);
    }
}

package org.qubership.routes.gateway.plugin;

import org.apache.maven.plugin.AbstractMojo;
import org.apache.maven.plugin.MojoExecutionException;
import org.apache.maven.plugins.annotations.LifecyclePhase;
import org.apache.maven.plugins.annotations.Mojo;
import org.apache.maven.plugins.annotations.Parameter;
import org.apache.maven.project.MavenProject;

import java.nio.file.Files;
import java.util.List;
import java.util.Set;

@Mojo(
        name = "generate",
        defaultPhase = LifecyclePhase.GENERATE_RESOURCES,
        threadSafe = true,
        aggregator = true
)
public class RouteToGatewayMojo extends AbstractMojo {

    @Parameter(defaultValue = "${reactorProjects}", readonly = true, required = true)
    private List<MavenProject> reactorProjects;

    @Parameter(defaultValue = "${project}", readonly = true)
    private MavenProject project;

    @Parameter(defaultValue = "com.netcracker")
    private String[] packages;

    @Parameter(defaultValue = "8080", required = false)
    private int servicePort;

    @Override
    public void execute() throws MojoExecutionException {
        RouteScanner scanner = new RouteScanner(packages, getLog());
        Set<HttpRoute> allRoutes = scanner.collectRoutes(reactorProjects);
        writeRoutesFile(allRoutes);
    }

    private void writeRoutesFile(Set<HttpRoute> routes) throws MojoExecutionException {
        try {
            java.nio.file.Path file = project.getBasedir()
                    .toPath()
                    .resolve("gateway-httproutes.yaml");

            String yaml = HttpRouteGenerator
                    .generateHttpRoutesYaml(servicePort, routes);

            Files.writeString(file, prependYamlHeader(yaml));
            getLog().info("Generated gateway-httproutes.yaml at root project");

        } catch (Exception e) {
            throw new MojoExecutionException("Failed to generate routes", e);
        }
    }

    private String prependYamlHeader(String yamlContent) {
        return """
                # -----------------------------------------------------------------------------
                # THIS FILE WAS AUTOMATICALLY GENERATED — DO NOT EDIT.
                # Any changes will be overwritten during the next build.
                # Modify source annotations and regenerate using RouteToGatewayMojo.
                # -----------------------------------------------------------------------------

                """ + yamlContent;
    }

}

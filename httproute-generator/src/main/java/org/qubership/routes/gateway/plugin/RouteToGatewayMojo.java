package org.qubership.routes.gateway.plugin;

import com.netcracker.cloud.routesregistration.common.annotation.FacadeRoute;
import com.netcracker.cloud.routesregistration.common.annotation.Route;
import com.netcracker.cloud.routesregistration.common.spring.gateway.route.annotation.GatewayRequestMapping;
import io.github.classgraph.*;
import org.apache.maven.plugin.AbstractMojo;
import org.apache.maven.plugin.MojoExecutionException;
import org.apache.maven.plugins.annotations.LifecyclePhase;
import org.apache.maven.plugins.annotations.Mojo;
import org.apache.maven.plugins.annotations.Parameter;
import org.apache.maven.project.MavenProject;
import org.springframework.web.bind.annotation.*;

import java.io.File;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.*;
import java.util.stream.Collectors;
import java.util.stream.Stream;

@Mojo(name = "generate",
        defaultPhase = LifecyclePhase.GENERATE_RESOURCES,
        threadSafe = true)
public class RouteToGatewayMojo extends AbstractMojo {

    @Parameter(defaultValue = "${project}", readonly = true)
    private MavenProject project;

    @Parameter(defaultValue = "com.netcracker")
    private String[] packages;

    @Parameter(defaultValue = "8080", required = false)
    private int servicePort;

    @Override
    public void execute() throws MojoExecutionException {
        Set<HttpRoute> routes = getRoutes();
        getLog().info("Routes: " + routes);
        try {
            getLog().info(project.getFile().getAbsolutePath());
            Path file = project.getBasedir().toPath().resolve("gateway-httproutes.yaml");
            String httpRoutesYaml = HttpRouteGenerator.generateHttpRoutesYaml(servicePort, routes);
            Files.writeString(file, prependYamlHeader(httpRoutesYaml));
        } catch (Exception e) {
            throw new RuntimeException(e);
        }
    }

    private Set<HttpRoute> getRoutes() throws MojoExecutionException {
        File classesDir = new File(project.getBuild().getOutputDirectory());
        if (!classesDir.exists()) {
            getLog().warn("No classes to scan: outputDirectory does not exist.");
            return Collections.emptySet();
        }

        try (ScanResult scan = new ClassGraph()
                .enableAllInfo()
                .overrideClasspath(classesDir.getAbsolutePath())
                .acceptPackages(packages)
                .scan()) {

            return Stream.of(
                            scan.getClassesWithMethodAnnotation(Route.class),
                            scan.getClassesWithMethodAnnotation(FacadeRoute.class),
                            scan.getClassesWithAnnotation(Route.class),
                            scan.getClassesWithAnnotation(FacadeRoute.class)
                    )
                    .flatMap(Collection::stream)
                    .distinct()
                    .map(this::getRequestMappingPaths)
                    .flatMap(Collection::stream)
                    .collect(Collectors.toSet());
        } catch (Exception e) {
            throw new MojoExecutionException("Failed scanning annotations", e);
        }
    }

    private List<String> getAnnotationPathFor(AnnotationInfo annotationInfo) {
        if (annotationInfo == null || annotationInfo.getParameterValues() == null || annotationInfo.getParameterValues().isEmpty()) {
            return Collections.emptyList();
        }
        String[] paths = (String[]) annotationInfo.getParameterValues().getValue("path");
        if (paths == null || paths.length == 0) {
            paths = (String[]) annotationInfo.getParameterValues().getValue("value");
        }
        return paths == null ? Collections.emptyList() : Arrays.asList(paths);
    }

    protected Set<HttpRoute> getRequestMappingPaths(ClassInfo classInfo) {
        getLog().info("Get Request Mappings for Class: " + classInfo.getName());

        Optional<HttpRoute.Type> classRouteType =
                getRouteType(classInfo.getAnnotationInfo(Route.class.getName()));

        List<String> classGatewayRequestMapping =
                getAnnotationPathFor(classInfo.getAnnotationInfo(GatewayRequestMapping.class.getName()));

        List<String> classesReqMappings = Optional.ofNullable(classInfo.getAnnotationInfo(RequestMapping.class))
                .or(() -> Optional.ofNullable(classInfo.getAnnotationInfo(GetMapping.class)))
                .or(() -> Optional.ofNullable(classInfo.getAnnotationInfo(PostMapping.class)))
                .or(() -> Optional.ofNullable(classInfo.getAnnotationInfo(PutMapping.class)))
                .or(() -> Optional.ofNullable(classInfo.getAnnotationInfo(DeleteMapping.class)))
                .or(() -> Optional.ofNullable(classInfo.getAnnotationInfo(PatchMapping.class)))
                .map(this::getAnnotationPathFor)
                .orElse(Collections.emptyList());

        Set<HttpRoute> routes = classInfo.getMethodInfo().stream()
                .flatMap(methodInfo ->
                        methodInfo.getAnnotationInfo().stream()
                                .filter(ann -> ann.getName().equals(RequestMapping.class.getName())
                                        || ann.getName().equals(GetMapping.class.getName())
                                        || ann.getName().equals(PostMapping.class.getName())
                                        || ann.getName().equals(PutMapping.class.getName())
                                        || ann.getName().equals(DeleteMapping.class.getName())
                                        || ann.getName().equals(PatchMapping.class.getName()))
                                .map(mappingAnn -> Map.entry(methodInfo, mappingAnn))
                )
                .map(entry -> {
                    AnnotationInfo mappingAnn = entry.getValue();
                    HttpRoute.Type routeType = getRouteType(
                            entry.getKey().getAnnotationInfo(Route.class.getName())
                    ).orElse(classRouteType.orElse(HttpRoute.Type.INTERNAL));

                    List<String> methodGatewayRequestMapping = getAnnotationPathFor(entry.getKey().getAnnotationInfo(GatewayRequestMapping.class.getName()));
                    if (!classGatewayRequestMapping.isEmpty()) {
                        return classGatewayRequestMapping.stream()
                                .flatMap(classPrefix -> methodGatewayRequestMapping.stream()
                                        .map(methodPath -> new HttpRoute(classesReqMappings.getFirst() + getAnnotationPathFor(mappingAnn).getFirst(), classPrefix + methodPath, routeType)))
                                .collect(Collectors.toSet());
                    } else if (!methodGatewayRequestMapping.isEmpty()) {
                        String classPrefix;
                        if (!classesReqMappings.isEmpty()) {
                            classPrefix = classesReqMappings.getFirst();
                        } else {
                            classPrefix = "";
                        }
                        return methodGatewayRequestMapping.stream()
                                .map(methodPath -> new HttpRoute(classPrefix + getAnnotationPathFor(mappingAnn).getFirst(), methodPath, routeType))
                                .collect(Collectors.toSet());
                    }

                    // No class prefix → direct paths
                    if (classesReqMappings.isEmpty()) {
                        return getAnnotationPathFor(mappingAnn).stream()
                                .map(path -> new HttpRoute(path, routeType))
                                .collect(Collectors.toSet());
                    }

                    // Merge class + method mappings
                    return classesReqMappings.stream()
                            .flatMap(classPrefix -> getAnnotationPathFor(mappingAnn).stream()
                                    .map(methodPath -> new HttpRoute(classPrefix + methodPath, routeType)))
                            .collect(Collectors.toSet());
                })
                .flatMap(Collection::stream)
                .collect(Collectors.toSet());

        getLog().info("Found " + routes.size() + " routes");
        if (classInfo.getSuperclass() != null) {
            routes.addAll(getRequestMappingPaths(classInfo.getSuperclass()));

        }

        return routes;
    }


    private Optional<HttpRoute.Type> getRouteType(AnnotationInfo annotationInfo) {
        return Optional.ofNullable(annotationInfo)
                .map(annInfo -> annInfo.getParameterValues(false))
                .flatMap(params ->
                        Optional.ofNullable(params.getValue("type"))
                                .or(() -> Optional.ofNullable(params.getValue("value")))
                )
                .filter(v -> v instanceof AnnotationEnumValue)
                .map(v -> (AnnotationEnumValue) v)
                .map(enumVal -> HttpRoute.Type.valueOf(enumVal.getValueName()))
                .or(() -> Optional.of(HttpRoute.Type.INTERNAL));
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

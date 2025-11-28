package org.qubership.routes.gateway.plugin;

import com.netcracker.cloud.routesregistration.common.annotation.FacadeRoute;
import com.netcracker.cloud.routesregistration.common.annotation.Route;
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

    @Parameter(required = true)
    private String serviceName;

    @Parameter(defaultValue = "com.netcracker")
    private String[] packages;

    @Override
    public void execute() throws MojoExecutionException {
        Set<HttpRoute> routes = getRoutes();
        getLog().info("Routes: " + routes);
        try {
            getLog().info(project.getFile().getAbsolutePath());
            Path file = project.getBasedir().toPath().resolve("gateway-httproutes.yaml");
            Files.writeString(file, HttpRouteGenerator.generateHttpRoutesYaml(serviceName, routes));
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
                    .flatMap(m -> m.entrySet().stream())
                    .flatMap(e -> e.getValue().stream()
                            .map(path -> new HttpRoute(path, e.getKey())))
                    .collect(Collectors.toSet());
        } catch (Exception e) {
            throw new MojoExecutionException("Failed scanning annotations", e);
        }
    }

    private List<String> getAnnotationPathFor(AnnotationInfo annotationInfo) {
        if (annotationInfo.getParameterValues().isEmpty()) {
            return Collections.emptyList();
        }
        return Optional.ofNullable(annotationInfo.getParameterValues().getValue("path"))
                .or(() -> Optional.ofNullable(annotationInfo.getParameterValues().getValue("value")))
                .filter(path -> path instanceof Object[])
                .map(o -> (Object[]) o)
                .map(o -> Arrays.stream(o).map(Object::toString).toList())
                .orElse(Collections.emptyList());
    }

    private Map<HttpRoute.Type, List<String>> getRequestMappingPaths(ClassInfo classInfo) {
        getLog().info("Get Request Mappings for Class: " + classInfo.getName());

        Optional<HttpRoute.Type> methodRouteType = getRouteType(classInfo.getAnnotationInfo(Route.class.getName()));

        List<String> classesReqMappings = Optional.ofNullable(classInfo.getAnnotationInfo(RequestMapping.class))
                .or(() -> Optional.ofNullable(classInfo.getAnnotationInfo(GetMapping.class)))
                .or(() -> Optional.ofNullable(classInfo.getAnnotationInfo(PostMapping.class)))
                .or(() -> Optional.ofNullable(classInfo.getAnnotationInfo(PutMapping.class)))
                .or(() -> Optional.ofNullable(classInfo.getAnnotationInfo(DeleteMapping.class)))
                .or(() -> Optional.ofNullable(classInfo.getAnnotationInfo(PatchMapping.class)))
                .map(this::getAnnotationPathFor)
                .orElse(Collections.emptyList());

        Map<HttpRoute.Type, List<String>> routes = classInfo.getMethodInfo().stream()
                .flatMap(methodInfo ->
                        methodInfo.getAnnotationInfo().stream()
                                .filter(ann -> ann.getName().equals(RequestMapping.class.getName())
                                        || ann.getName().equals(GetMapping.class.getName())
                                        || ann.getName().equals(PostMapping.class.getName())
                                        || ann.getName().equals(PutMapping.class.getName())
                                        || ann.getName().equals(DeleteMapping.class.getName())
                                        || ann.getName().equals(PatchMapping.class.getName()))
                                .map(mappingAnnotation -> Map.entry(methodInfo, mappingAnnotation))
                )
                .collect(Collectors.toMap(
                        entry -> {
                            MethodInfo method = entry.getKey();
                            AnnotationInfo routeAnn = method.getAnnotationInfo(Route.class.getName());
                            return methodRouteType.orElse(
                                    getRouteType(routeAnn).orElse(HttpRoute.Type.INTERNAL)
                            );
                        },
                        entry -> {
                            AnnotationInfo mappingAnn = entry.getValue();
                            return classesReqMappings.stream()
                                    .flatMap(classPrefix -> getAnnotationPathFor(mappingAnn).stream()
                                            .map(methodPath -> classPrefix + "/" + methodPath))
                                    .toList();
                        },
                        (list1, list2) -> {
                            List<String> merged = new ArrayList<>(list1);
                            merged.addAll(list2);
                            return merged;
                        }
                ));
        getLog().info("Found " + routes.size() + " routes");
        return routes;
    }

    private Optional<HttpRoute.Type> getRouteType(AnnotationInfo annotationInfo) {
        return Optional.ofNullable(annotationInfo)
                .map(annInfo -> annInfo.getParameterValues().getValue("type"))
                .map(o -> HttpRoute.Type.valueOf(((AnnotationEnumValue) o).getValueName()));
    }
}

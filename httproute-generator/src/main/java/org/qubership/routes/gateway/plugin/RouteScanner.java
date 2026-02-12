package org.qubership.routes.gateway.plugin;

import com.netcracker.cloud.routesregistration.common.annotation.Gateway;
import com.netcracker.cloud.routesregistration.common.annotation.Route;
import com.netcracker.cloud.routesregistration.common.spring.gateway.route.annotation.GatewayRequestMapping;
import io.github.classgraph.*;
import jakarta.ws.rs.*;
import org.apache.maven.plugin.MojoExecutionException;
import org.apache.maven.plugin.logging.Log;
import org.apache.maven.project.MavenProject;
import org.springframework.web.bind.annotation.*;

import java.io.File;
import java.util.*;
import java.util.stream.Collectors;
import java.util.stream.Stream;


/**
 * Scans compiled classes to collect HttpRoute definitions based on annotations.
 */
public class RouteScanner {

    private final String[] packages;
    private final Log log;

    public RouteScanner(String[] packages, Log log) {
        this.packages = packages;
        this.log = log;
    }

    public Set<HttpRoute> collectRoutes(List<MavenProject> reactorProjects) throws MojoExecutionException {
        Set<HttpRoute> allRoutes = new HashSet<>();
        for (MavenProject module : reactorProjects) {
            log.info("Scanning module: " + module.getArtifactId());
            allRoutes.addAll(getRoutes(module));
        }
        return allRoutes;
    }

    public Set<HttpRoute> getRoutes(MavenProject module) throws MojoExecutionException {
        File classesDir = new File(module.getBuild().getOutputDirectory());
        if (!classesDir.exists()) {
            log.warn("No classes to scan: outputDirectory does not exist.");
            return Collections.emptySet();
        }

        try (ScanResult scan = new ClassGraph()
                .enableAllInfo()
                .overrideClasspath(classesDir.getAbsolutePath())
                .acceptPackages(packages)
                .disableRuntimeInvisibleAnnotations()
                .scan()) {

            Stream<ClassInfoList> pathsStream;
            if (isSpringUsed(scan)) {
                pathsStream = Stream.of(
                        scan.getClassesWithMethodAnnotation(RequestMapping.class),
                        scan.getClassesWithAnnotation(RequestMapping.class),
                        scan.getClassesWithMethodAnnotation(GetMapping.class),
                        scan.getClassesWithAnnotation(GetMapping.class),
                        scan.getClassesWithMethodAnnotation(PostMapping.class),
                        scan.getClassesWithAnnotation(PostMapping.class),
                        scan.getClassesWithMethodAnnotation(PutMapping.class),
                        scan.getClassesWithAnnotation(DeleteMapping.class),
                        scan.getClassesWithMethodAnnotation(DeleteMapping.class),
                        scan.getClassesWithAnnotation(PutMapping.class),
                        scan.getClassesWithMethodAnnotation(PatchMapping.class),
                        scan.getClassesWithAnnotation(PatchMapping.class)
                );
            } else if (isQuarkusUsed(scan)) {
                pathsStream = Stream.of(
                        scan.getClassesWithMethodAnnotation(Path.class),
                        scan.getClassesWithAnnotation(Path.class),
                        scan.getClassesWithMethodAnnotation(GET.class),
                        scan.getClassesWithAnnotation(GET.class),
                        scan.getClassesWithMethodAnnotation(POST.class),
                        scan.getClassesWithAnnotation(POST.class),
                        scan.getClassesWithMethodAnnotation(PUT.class),
                        scan.getClassesWithAnnotation(PUT.class),
                        scan.getClassesWithMethodAnnotation(DELETE.class),
                        scan.getClassesWithAnnotation(DELETE.class),
                        scan.getClassesWithMethodAnnotation(PATCH.class),
                        scan.getClassesWithAnnotation(PATCH.class)
                );
            } else {
                return Set.of();
            }

            return pathsStream
                    .flatMap(Collection::stream)
                    .distinct()
                    .filter(this::hasRoute)
                    .map(this::getRequestMappingPaths)
                    .flatMap(Collection::stream)
                    .collect(Collectors.toSet());
        } catch (Exception e) {
            throw new MojoExecutionException("Failed scanning annotations", e);
        }
    }

    public Set<HttpRoute> getRequestMappingPaths(ClassInfo classInfo) {
        log.info("Get Request Mappings for Class: " + classInfo.getName());

        Optional<HttpRoute.Type> classRouteType = getRouteType(classInfo.getAnnotationInfo(Route.class.getName()));
        Optional<Long> classRouteTimeout = getRouteTimeout(classInfo.getAnnotationInfo(Route.class.getName()));

        List<String> classGatewayRequestMapping = resolveGatewayMappings(classInfo);
        List<String> classesReqMappings = resolveRequestMappings(classInfo);

        Set<HttpRoute> routes = classInfo.getMethodInfo().stream()
                .flatMap(methodInfo -> methodInfo.getAnnotationInfo().stream()
                        .filter(annotationInfo -> isHttpMappingAnnotation(annotationInfo) || annotationInfo.getName().equals(RequestMapping.class.getName()))
                        .filter(annotationInfo -> !isMetaRequestMapping(annotationInfo, methodInfo))
                        .map(mappingAnn -> Map.entry(methodInfo, mappingAnn)))
                .map(entry -> buildRoutesForMethod(
                        entry.getKey(),
                        entry.getValue(),
                        classRouteType,
                        classRouteTimeout,
                        classGatewayRequestMapping,
                        classesReqMappings))
                .flatMap(Collection::stream)
                .collect(Collectors.toSet());

        if (classInfo.getSuperclass() != null) {
            routes.addAll(getRequestMappingPaths(classInfo.getSuperclass()));
        }

        log.info("Found " + routes.size() + " routes");
        return routes;
    }

    private boolean hasRoute(ClassInfo classInfo) {
        return classInfo.hasAnnotation(Route.class) || classInfo.hasMethodAnnotation(Route.class);
    }

    private boolean isMetaRequestMapping(AnnotationInfo annotationInfo, MethodInfo methodInfo) {
        String annotationName = annotationInfo.getName();

        // Special handling for @RequestMapping - it can be a meta-annotation on @GetMapping, @PostMapping, etc.
        return RequestMapping.class.getName().equals(annotationName) && methodInfo.getAnnotationInfo().stream().anyMatch(this::isHttpMappingAnnotation);
    }

    private List<String> resolveGatewayMappings(ClassInfo classInfo) {
        if (classInfo.hasAnnotation(GatewayRequestMapping.class.getName())) {
            return getAnnotationPathFor(classInfo.getAnnotationInfo(GatewayRequestMapping.class.getName()));
        }
        return getAnnotationPathFor(classInfo.getAnnotationInfo(Gateway.class.getName()));
    }

    private List<String> resolveGatewayMappings(MethodInfo methodInfo) {
        if (methodInfo.hasAnnotation(GatewayRequestMapping.class.getName())) {
            return getAnnotationPathFor(methodInfo.getAnnotationInfo(GatewayRequestMapping.class.getName()));
        }
        return getAnnotationPathFor(methodInfo.getAnnotationInfo(Gateway.class.getName()));
    }

    private List<String> resolveRequestMappings(ClassInfo classInfo) {
        return Optional.ofNullable(classInfo.getAnnotationInfo(RequestMapping.class))
                .or(() -> Optional.ofNullable(classInfo.getAnnotationInfo(GetMapping.class)))
                .or(() -> Optional.ofNullable(classInfo.getAnnotationInfo(PostMapping.class)))
                .or(() -> Optional.ofNullable(classInfo.getAnnotationInfo(PutMapping.class)))
                .or(() -> Optional.ofNullable(classInfo.getAnnotationInfo(DeleteMapping.class)))
                .or(() -> Optional.ofNullable(classInfo.getAnnotationInfo(PatchMapping.class)))
                .or(() -> Optional.ofNullable(classInfo.getAnnotationInfo(Path.class)))
                .map(this::getAnnotationPathFor)
                .orElse(Collections.emptyList());
    }

    private boolean isHttpMappingAnnotation(AnnotationInfo annotationInfo) {
        String name = annotationInfo.getName();
        return name.equals(GetMapping.class.getName())
                || name.equals(PostMapping.class.getName())
                || name.equals(PutMapping.class.getName())
                || name.equals(DeleteMapping.class.getName())
                || name.equals(PatchMapping.class.getName())
                || name.equals(GET.class.getName())
                || name.equals(POST.class.getName())
                || name.equals(PUT.class.getName())
                || name.equals(DELETE.class.getName())
                || name.equals(PATCH.class.getName());
    }

    private Set<HttpRoute> buildRoutesForMethod(
            MethodInfo methodInfo,
            AnnotationInfo mappingAnn,
            Optional<HttpRoute.Type> classRouteType,
            Optional<Long> classRouteTimeout,
            List<String> classGatewayRequestMapping,
            List<String> classesReqMappings
    ) {
        HttpRoute.Type routeType = getRouteType(methodInfo.getAnnotationInfo(Route.class.getName()))
                .orElse(classRouteType.orElse(HttpRoute.Type.INTERNAL));
        long routeTimeout = getRouteTimeout(methodInfo.getAnnotationInfo(Route.class.getName()))
                .orElse(classRouteTimeout.orElse(0L));

        List<String> methodGatewayRequestMapping = resolveGatewayMappings(methodInfo);
        List<String> mappingPaths;
        if (mappingAnn.getName().equals(POST.class.getName())) {
            mappingPaths = getAnnotationPathFor(methodInfo.getAnnotationInfo(Path.class.getName()));
        } else {
            mappingPaths = getAnnotationPathFor(mappingAnn);
        }

        if (!classGatewayRequestMapping.isEmpty()) {
            return buildClassGatewayRoutes(classGatewayRequestMapping, methodGatewayRequestMapping, classesReqMappings, mappingPaths, routeType, routeTimeout);
        }
        if (!methodGatewayRequestMapping.isEmpty()) {
            return buildMethodGatewayRoutes(methodGatewayRequestMapping, classesReqMappings, mappingPaths, routeType, routeTimeout);
        }

        if (classesReqMappings.isEmpty()) {
            return mappingPaths.stream()
                    .map(path -> new HttpRoute(path, routeType, routeTimeout))
                    .collect(Collectors.toSet());
        }

        return classesReqMappings.stream()
                .flatMap(classPrefix -> mappingPaths.stream()
                        .map(methodPath -> new HttpRoute(classPrefix + methodPath, routeType, routeTimeout)))
                .collect(Collectors.toSet());
    }

    private Set<HttpRoute> buildClassGatewayRoutes(
            List<String> classGatewayRequestMapping,
            List<String> methodGatewayRequestMapping,
            List<String> classesReqMappings,
            List<String> methodReqMappings,
            HttpRoute.Type routeType,
            long routeTimeout
    ) {
        if (methodReqMappings.isEmpty()) {
            methodReqMappings = List.of("/");
        }
        if (methodGatewayRequestMapping.isEmpty()) {
            methodGatewayRequestMapping = methodReqMappings;
        }
        String servicePrefix = classesReqMappings.getFirst();
        String mappingPath = methodReqMappings.isEmpty() ? "" : methodReqMappings.getFirst();
        List<String> finalMethodGatewayRequestMapping = methodGatewayRequestMapping;
        return classGatewayRequestMapping.stream()
                .flatMap(classPrefix -> finalMethodGatewayRequestMapping.stream()
                        .map(methodPath -> new HttpRoute(
                                servicePrefix + mappingPath,
                                classPrefix + methodPath,
                                routeType,
                                routeTimeout
                        )))
                .collect(Collectors.toSet());
    }

    private Set<HttpRoute> buildMethodGatewayRoutes(
            List<String> methodGatewayRequestMapping,
            List<String> classesReqMappings,
            List<String> mappingPaths,
            HttpRoute.Type routeType,
            long routeTimeout
    ) {
        if (methodGatewayRequestMapping.isEmpty() || mappingPaths.isEmpty()) {
            return Collections.emptySet();
        }

        String servicePrefix = classesReqMappings.isEmpty() ? "" : classesReqMappings.getFirst();
        String mappingPath = mappingPaths.getFirst();
        return methodGatewayRequestMapping.stream()
                .map(methodPath -> new HttpRoute(servicePrefix + mappingPath, methodPath, routeType, routeTimeout))
                .collect(Collectors.toSet());
    }

    private List<String> getAnnotationPathFor(AnnotationInfo annotationInfo) {
        if (annotationInfo == null) {
            return Collections.emptyList();
        }

        AnnotationParameterValueList parameters = annotationInfo.getParameterValues();

        // Handle single string value
        Object valueParam = parameters.getValue("value");
        Object pathParam = parameters.getValue("path");

        if (isNullOrEmpty(valueParam) && isNullOrEmpty(pathParam)) {
            return List.of("");
        }
        if (valueParam instanceof String && !isNullOrEmpty(valueParam)) {
            return List.of(valueParam.toString());
        }
        if (pathParam instanceof String && !isNullOrEmpty(pathParam)) {
            return List.of(pathParam.toString());
        }

        // Try "path" parameter first, then fall back to "value"
        AnnotationParameterValueList finalParameters = parameters;
        return extractPathsFromParameter(parameters, "path")
                .or(() -> extractPathsFromParameter(finalParameters, "value"))
                .orElse(List.of(""));
    }

    private boolean isNullOrEmpty(Object param) {
        return switch (param) {
            case null -> true;
            case String s -> s.isEmpty();
            case Object[] objects -> objects.length == 0;
            default -> false;
        };
    }

    private boolean hasNoParameters(AnnotationInfo annotationInfo) {
        return annotationInfo.getParameterValues() == null
                || annotationInfo.getParameterValues().isEmpty();
    }

    private Optional<List<String>> extractPathsFromParameter(AnnotationParameterValueList parameters, String parameterName) {
        Object paramValue = parameters.getValue(parameterName);

        switch (paramValue) {
            case String s -> {
                return Optional.of(List.of(s));
            }
            case Object[] objects -> {
                if (objects.length == 0) {
                    return Optional.empty();
                }

                List<String> paths = Arrays.stream(objects)
                        .filter(String.class::isInstance)
                        .map(String.class::cast)
                        .toList();

                return paths.isEmpty() ? Optional.empty() : Optional.of(paths);
            }
            case null, default -> {
                return Optional.empty();
            }
        }
    }

    private Optional<Long> getRouteTimeout(AnnotationInfo annotationInfo) {
        return Optional.ofNullable(annotationInfo)
                .map(a -> a.getParameterValues(false))
                .map(p -> p.getValue("timeout"))
                .filter(Number.class::isInstance)
                .map(Number.class::cast)
                .map(Number::longValue);
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
                .map(enumVal -> HttpRoute.Type.valueOf(enumVal.getValueName()));
    }

    private boolean isSpringUsed(ScanResult scan) {
        return !scan.getClassesWithMethodAnnotation(RequestMapping.class).isEmpty()
                || !scan.getClassesWithAnnotation(RequestMapping.class).isEmpty()
                || !scan.getClassesWithMethodAnnotation(GetMapping.class).isEmpty()
                || !scan.getClassesWithAnnotation(GetMapping.class).isEmpty()
                || !scan.getClassesWithMethodAnnotation(PostMapping.class).isEmpty()
                || !scan.getClassesWithAnnotation(PostMapping.class).isEmpty()
                || !scan.getClassesWithMethodAnnotation(PutMapping.class).isEmpty()
                || !scan.getClassesWithAnnotation(PutMapping.class).isEmpty()
                || !scan.getClassesWithMethodAnnotation(DeleteMapping.class).isEmpty()
                || !scan.getClassesWithAnnotation(DeleteMapping.class).isEmpty()
                || !scan.getClassesWithMethodAnnotation(PatchMapping.class).isEmpty()
                || !scan.getClassesWithAnnotation(PatchMapping.class).isEmpty();
    }

    private boolean isQuarkusUsed(ScanResult scan) {
        return !scan.getClassesWithMethodAnnotation(Path.class).isEmpty()
                || !scan.getClassesWithAnnotation(Path.class).isEmpty();
    }
}

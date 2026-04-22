package com.netcracker.it.controlplane;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.netcracker.cloud.junit.cloudcore.extension.annotations.Cloud;
import com.netcracker.cloud.junit.cloudcore.extension.annotations.EnableExtension;
import com.netcracker.cloud.junit.cloudcore.extension.annotations.PortForward;
import com.netcracker.cloud.junit.cloudcore.extension.annotations.Value;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.client.KubernetesClient;
import lombok.extern.slf4j.Slf4j;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.Response;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;
import io.fabric8.kubernetes.client.dsl.ExecWatch;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.net.URL;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

@Slf4j
@EnableExtension
public class ControlPlaneRoutesIT {

    @PortForward(serviceName = @Value("internal-gateway-service"))
    protected static URL internalGateway;

    @PortForward(serviceName = @Value("private-gateway-service"))
    protected static URL privateGateway;

    @PortForward(serviceName = @Value("public-gateway-service"))
    protected static URL publicGateway;

    @Cloud
    protected static KubernetesClient kubernetesClient;

    protected static OkHttpClient okHttpClient;
    protected static ObjectMapper objectMapper;
    protected static String namespace;
    
    protected static Map<String, List<String>> expectedRoutesMap;
    protected static ConfigMap routesConfigMap;
    protected static String controlPlanePod;

    @BeforeAll
    public static void setUp() throws Exception {
        namespace = kubernetesClient.getNamespace();
        
        var pods = kubernetesClient.pods()
                .inNamespace(namespace)
                .withLabel("app", "control-plane")
                .list();
        
        if (pods.getItems().isEmpty()) {
            pods = kubernetesClient.pods()
                    .inNamespace(namespace)
                    .withLabel("name", "control-plane")
                    .list();
        }

        objectMapper = new ObjectMapper();
        
        okHttpClient = new OkHttpClient.Builder()
                .readTimeout(30, TimeUnit.SECONDS)
                .connectTimeout(30, TimeUnit.SECONDS)
                .build();
        
        loadExpectedRoutesFromConfigMap();
        
        if (!pods.getItems().isEmpty()) {
            controlPlanePod = pods.getItems().get(0).getMetadata().getName();
            log.info("Control-plane pod: {}", controlPlanePod);
        }

        log.info("Internal gateway URL: {}", internalGateway);
        log.info("Private gateway URL: {}", privateGateway);
        log.info("Public gateway URL: {}", publicGateway);
        log.info("Test namespace: {}", namespace);
        log.info("Loaded {} routes for internal gateway", expectedRoutesMap.get("internal").size());
        log.info("Loaded {} routes for private gateway", expectedRoutesMap.get("private").size());
        log.info("Loaded {} routes for public gateway", expectedRoutesMap.get("public").size());
    }

    /**
     * Executes command in control-plane pod and returns output
     */
    private static String execInControlPlane(String... command) throws Exception {
        ByteArrayOutputStream out = new ByteArrayOutputStream();
        ByteArrayOutputStream err = new ByteArrayOutputStream();
        
        log.info("Executing in pod {}: {}", controlPlanePod, String.join(" ", command));
        
        try (ExecWatch execWatch = kubernetesClient.pods()
                .inNamespace(namespace)
                .withName(controlPlanePod)
                .writingOutput(out)
                .writingError(err)
                .exec(command)) {
            
            // Wait for command to complete - exitCode() returns CompletableFuture
            Integer exitCode = execWatch.exitCode().get();
            
            String output = out.toString();
            String error = err.toString();
            
            log.info("Exit code: {}, Output: '{}', Error: '{}'", exitCode, output.trim(), error.trim());
            
            if (exitCode != null && exitCode != 0) {
                log.warn("Command failed with exit code {}: {}", exitCode, error);
            }
            
            return output;
        }
    }

    /**
     * Executes curl to internal gateway and returns status code
     */
    private static int curlInternalStatusCode(String path) throws Exception {
        String url = String.format("http://internal-gateway-service.%s.svc.cluster.local:8080%s", namespace, path);
        log.info("Curling: {}", url);
        
        String result = execInControlPlane("curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", url);
        
        if (result == null || result.trim().isEmpty()) {
            log.error("Empty response from curl for path: {}", path);
            return 999; // Return special code for empty response
        }
        
        try {
            return Integer.parseInt(result.trim());
        } catch (NumberFormatException e) {
            log.error("Failed to parse curl response: '{}' for path: {}", result, path);
            return 999;
        }
    }

    /**
     * Executes curl to internal gateway and returns full response
     */
    private static String curlInternalWithBody(String path) throws Exception {
        String url = String.format("http://internal-gateway-service.%s.svc.cluster.local:8080%s", namespace, path);
        return execInControlPlane("curl", "-s", url);
    }
    
    @BeforeEach
    public void createRoutesConfigMap() {
        // Create or update ConfigMap in Kubernetes before tests
        if (routesConfigMap == null) {
            routesConfigMap = loadConfigMapFromResource();
            kubernetesClient.configMaps()
                    .inNamespace(namespace)
                    .resource(routesConfigMap)
                    .createOrReplace();
            log.info("Created/Updated ConfigMap 'expected-routes' in namespace {}", namespace);
        }
    }
    
    private static void loadExpectedRoutesFromConfigMap() throws IOException {
        // First try to get from Kubernetes (if already exists)
        ConfigMap existingCm = kubernetesClient.configMaps()
                .inNamespace(namespace)
                .withName("expected-routes")
                .get();
        
        if (existingCm != null && existingCm.getData() != null) {
            log.info("Loading expected routes from existing ConfigMap in cluster");
            expectedRoutesMap = parseRoutesFromConfigMap(existingCm);
        } else {
            // Load from classpath resource
            log.info("Loading expected routes from classpath resource");
            expectedRoutesMap = loadRoutesFromResource();
        }
    }
    
    private static ConfigMap loadConfigMapFromResource() {
        try (InputStream is = ControlPlaneRoutesIT.class.getResourceAsStream("/expected-routes.yaml")) {
            if (is == null) {
                log.warn("expected-routes.yaml not found, creating default ConfigMap");
                return createDefaultConfigMap();
            }
            
            // Load as ConfigMap from YAML
            return kubernetesClient.configMaps()
                    .load(is)
                    .item();
        } catch (IOException e) {
            log.error("Failed to load ConfigMap from resource", e);
            return createDefaultConfigMap();
        }
    }
    
    private static Map<String, List<String>> loadRoutesFromResource() throws IOException {
        // Try JSON format first
        try (InputStream is = ControlPlaneRoutesIT.class.getResourceAsStream("/expected-routes.json")) {
            if (is != null) {
                return objectMapper.readValue(is, new TypeReference<Map<String, List<String>>>() {});
            }
        }
        
        // Try YAML format
        try (InputStream is = ControlPlaneRoutesIT.class.getResourceAsStream("/expected-routes.yaml")) {
            if (is != null) {
                // Parse YAML ConfigMap
                ConfigMap cm = kubernetesClient.configMaps().load(is).item();
                return parseRoutesFromConfigMap(cm);
            }
        }
        
        // Return default routes if no resource found
        log.warn("No routes resource found, using default routes");
        return getDefaultRoutes();
    }
    
    private static Map<String, List<String>> parseRoutesFromConfigMap(ConfigMap cm) {
        Map<String, List<String>> routes = new java.util.HashMap<>();
        
        if (cm.getData() != null) {
            // Check for JSON format
            if (cm.getData().containsKey("routes.json")) {
                try {
                    String json = cm.getData().get("routes.json");
                    return objectMapper.readValue(json, new TypeReference<Map<String, List<String>>>() {});
                } catch (IOException e) {
                    log.error("Failed to parse routes.json", e);
                }
            }
            
            // Check for individual route lists per gateway
            for (String gateway : List.of("internal", "private", "public")) {
                String key = gateway + "-routes";
                if (cm.getData().containsKey(key)) {
                    String routesStr = cm.getData().get(key);
                    List<String> routeList = List.of(routesStr.split("\\n"));
                    routeList = routeList.stream()
                            .filter(s -> !s.trim().isEmpty())
                            .map(String::trim)
                            .toList();
                    routes.put(gateway, routeList);
                }
            }
        }
        
        // Fill missing gateways with defaults
        if (!routes.containsKey("internal")) {
            routes.put("internal", getDefaultInternalRoutes());
        }
        if (!routes.containsKey("private")) {
            routes.put("private", getDefaultPrivateRoutes());
        }
        if (!routes.containsKey("public")) {
            routes.put("public", getDefaultPublicRoutes());
        }
        
        return routes;
    }
    
    private static ConfigMap createDefaultConfigMap() {
        ConfigMap cm = new ConfigMap();
        cm.setMetadata(new io.fabric8.kubernetes.api.model.ObjectMeta());
        cm.getMetadata().setName("expected-routes");
        cm.setData(new java.util.HashMap<>());
        
        try {
            cm.getData().put("routes.json", objectMapper.writeValueAsString(getDefaultRoutes()));
        } catch (IOException e) {
            log.error("Failed to create default routes JSON", e);
        }
        
        return cm;
    }
    
    private static Map<String, List<String>> getDefaultRoutes() {
        Map<String, List<String>> routes = new java.util.HashMap<>();
        routes.put("internal", getDefaultInternalRoutes());
        routes.put("private", getDefaultPrivateRoutes());
        routes.put("public", getDefaultPublicRoutes());
        return routes;
    }
    
    private static List<String> getDefaultInternalRoutes() {
        return List.of(
            "/api/v1/routes",
            "/api/v1/routes/test",
            "/api/v2/control-plane/routing/details",
            "/api/v3/control-plane/routing/details",
            "/health"
        );
    }
    
    private static List<String> getDefaultPrivateRoutes() {
        return List.of(
            "/api/v1/routes",
            "/api/v1/control-plane/routes/clusters",
            "/api/v2/control-plane/routes",
            "/api/v3/control-plane/routing/details",
            "/health"
        );
    }
    
    private static List<String> getDefaultPublicRoutes() {
        return List.of(
            "/api/v1/routes",
            "/api/v3/control-plane/ui",
            "/health"
        );
    }

    /**
     * Provides test cases for route validation from ConfigMap
     */
    static Stream<Arguments> routeTestCasesFromConfigMap() {
        List<Arguments> arguments = new ArrayList<>();
        
        for (Map.Entry<String, List<String>> entry : expectedRoutesMap.entrySet()) {
            String gatewayType = entry.getKey();
            for (String route : entry.getValue()) {
                arguments.add(Arguments.of(gatewayType, route, 200));
            }
        }
        
        // Add negative test cases
        arguments.add(Arguments.of("internal", "/non-existent-route-12345", 404));
        arguments.add(Arguments.of("private", "/non-existent-route-67890", 404));
        arguments.add(Arguments.of("public", "/non-existent-route-abcde", 404));
        
        return arguments.stream();
    }

    @ParameterizedTest(name = "Internal via exec: {0}")
    @MethodSource("internalRoutesSource")
    @DisplayName("Test internal gateway routes via exec from control-plane")
    void testInternalRouteViaExec(String path) throws Exception {
        int code = curlInternalStatusCode(path);
        log.info("Internal {} -> {}", path, code);
        
        // 405 Method Not Allowed - acceptable (route exists)
        if (code == 405) {
            log.info("Route exists but method not allowed");
            return;
        }
        
        assertTrue(code < 500, "Route should be accessible, got: " + code);
    }

    static Stream<String> internalRoutesSource() {
        return Stream.of(
            "/api/v1/routes",
            "/api/v1/routes/test",
            "/api/v2/control-plane/routing/details",
            "/api/v3/control-plane/routing/details",
            "/health"
        );
    }

    @ParameterizedTest(name = "[{0}] {1}")
    @MethodSource("routeTestCasesFromConfigMap")
    @DisplayName("Test routes via port-forward")
    void testRoutesFromConfigMap(String gatewayType, String path, int expectedStatus) throws IOException {
        if ("internal".equals(gatewayType)) {
            return;
        }
        
        URL gatewayUrl = getGatewayUrl(gatewayType);
        String fullUrl = buildUrl(gatewayUrl, path);
        
        Request request = new Request.Builder()
                .url(fullUrl)
                .get()
                .build();
        
        try (Response response = okHttpClient.newCall(request).execute()) {
            log.info("Testing route: {} -> Status: {}", fullUrl, response.code());
            
            if (response.code() == 405) {
                log.info("Route exists but method not allowed");
                return;
            }
            
            if (expectedStatus == 200) {
                assertTrue(response.code() != 404,
                    String.format("Route %s on %s gateway should be accessible, got %d", 
                        path, gatewayType, response.code()));
            } else {
                assertEquals(expectedStatus, response.code(),
                    String.format("Route %s on %s gateway returned unexpected status", path, gatewayType));
            }
        }
    }

    @Test
    @DisplayName("Verify all expected routes are configured")
    void verifyAllRoutesAccessible() throws Exception {
        List<String> failedRoutes = new ArrayList<>();
        
        for (String route : getDefaultInternalRoutes()) {
            int code = curlInternalStatusCode(route);
            if (code >= 500) {
                failedRoutes.add(String.format("internal:%s (status %d)", route, code));
            }
        }
        
        for (Map.Entry<String, List<String>> entry : expectedRoutesMap.entrySet()) {
            String gatewayType = entry.getKey();
            if ("internal".equals(gatewayType)) continue;
            
            URL gatewayUrl = getGatewayUrl(gatewayType);
            
            for (String route : entry.getValue()) {
                String fullUrl = buildUrl(gatewayUrl, route);
                Request request = new Request.Builder()
                        .url(fullUrl)
                        .get()
                        .build();
                
                try (Response response = okHttpClient.newCall(request).execute()) {
                    if (response.code() >= 500) {
                        failedRoutes.add(String.format("%s:%s (status %d)", gatewayType, route, response.code()));
                        log.error("Route failed: {} -> {}", fullUrl, response.code());
                    }
                } catch (IOException e) {
                    failedRoutes.add(String.format("%s:%s (error: %s)", gatewayType, route, e.getMessage()));
                    log.error("Route error: {} -> {}", fullUrl, e.getMessage());
                }
            }
        }
        
        assertTrue(failedRoutes.isEmpty(), 
            "Following routes failed: " + String.join(", ", failedRoutes));
    }

    private static String buildUrl(URL baseUrl, String path) {
        String base = baseUrl.toString();
        if (base.endsWith("/")) {
            base = base.substring(0, base.length() - 1);
        }
        if (!path.startsWith("/")) {
            path = "/" + path;
        }
        return base + path;
    }

    private URL getGatewayUrl(String gatewayType) {
        switch (gatewayType.toLowerCase()) {
            case "internal":
                return internalGateway;
            case "private":
                return privateGateway;
            case "public":
                return publicGateway;
            default:
                throw new IllegalArgumentException("Unknown gateway type: " + gatewayType);
        }
    }
}
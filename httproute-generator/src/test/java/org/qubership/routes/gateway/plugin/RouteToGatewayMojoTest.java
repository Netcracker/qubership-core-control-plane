package org.qubership.routes.gateway.plugin;

import io.github.classgraph.ClassGraph;
import io.github.classgraph.ClassInfo;
import io.github.classgraph.ScanResult;
import org.apache.maven.plugin.logging.Log;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.Mock;
import org.mockito.MockitoAnnotations;
import org.mockito.Spy;

import java.util.Map;
import java.util.Set;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.Mockito.when;

public class RouteToGatewayMojoTest {

    @Spy
    private RouteToGatewayMojo mojo;

    @Mock
    private Log mockLog;

    @BeforeEach
    void setUp() {
        MockitoAnnotations.openMocks(this);
        when(mojo.getLog()).thenReturn(mockLog);
    }

    @Test
    void testGetRequestMappingPaths_TestController1() {
        try (ScanResult scan = new ClassGraph()
                .enableAllInfo()
                .acceptClasses(TestController1.class.getName())
                .scan()) {

            ClassInfo info = scan.getClassInfo(TestController1.class.getName());
            Map<HttpRoute.Type, Set<String>> routes = mojo.getRequestMappingPaths(info);
            assertEquals(2, routes.size());

            assertTrue(routes.containsKey(HttpRoute.Type.INTERNAL));
            Set<String> internalRoutes = routes.get(HttpRoute.Type.INTERNAL);
            assertEquals(2, internalRoutes.size());
            assertTrue(internalRoutes.contains(RoutesTestConfiguration.CLASS_ROUTES_1 + RoutesTestConfiguration.METHOD_ROUTES_1 + RoutesTestConfiguration.METHOD_ROUTES_2));
            assertTrue(internalRoutes.contains(RoutesTestConfiguration.CLASS_ROUTES_2 + RoutesTestConfiguration.METHOD_ROUTES_1 + RoutesTestConfiguration.METHOD_ROUTES_2));

            assertTrue(routes.containsKey(HttpRoute.Type.PRIVATE));
            Set<String> privateRoutes = routes.get(HttpRoute.Type.PRIVATE);
            assertEquals(4, privateRoutes.size());
            assertTrue(privateRoutes.contains(RoutesTestConfiguration.CLASS_ROUTES_1 + RoutesTestConfiguration.METHOD_ROUTES_1));
            assertTrue(privateRoutes.contains(RoutesTestConfiguration.CLASS_ROUTES_1 + RoutesTestConfiguration.METHOD_ROUTES_2));
            assertTrue(privateRoutes.contains(RoutesTestConfiguration.CLASS_ROUTES_2 + RoutesTestConfiguration.METHOD_ROUTES_1));
            assertTrue(privateRoutes.contains(RoutesTestConfiguration.CLASS_ROUTES_2 + RoutesTestConfiguration.METHOD_ROUTES_2));
        }
    }

    @Test
    void testGetRequestMappingPaths_TestController2() {
        try (ScanResult scan = new ClassGraph()
                .enableAllInfo()
                .acceptClasses(TestController2.class.getName())
                .scan()) {

            ClassInfo info = scan.getClassInfo(TestController2.class.getName());
            Map<HttpRoute.Type, Set<String>> routes = mojo.getRequestMappingPaths(info);
            assertEquals(2, routes.size());

            assertTrue(routes.containsKey(HttpRoute.Type.INTERNAL));
            Set<String> internalRoutes = routes.get(HttpRoute.Type.INTERNAL);
            assertEquals(2, internalRoutes.size());
            assertTrue(internalRoutes.contains(RoutesTestConfiguration.METHOD_ROUTES_1));
            assertTrue(internalRoutes.contains(RoutesTestConfiguration.METHOD_ROUTES_2));

            assertTrue(routes.containsKey(HttpRoute.Type.PRIVATE));
            Set<String> privateRoutes = routes.get(HttpRoute.Type.PRIVATE);
            assertEquals(1, privateRoutes.size());
            assertTrue(privateRoutes.contains(RoutesTestConfiguration.METHOD_ROUTES_1 + RoutesTestConfiguration.METHOD_ROUTES_2));
        }
    }
}

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
            Set<HttpRoute> routes = mojo.getRequestMappingPaths(info);

            assertEquals(6, routes.size());
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTES_1 + RoutesTestConfiguration.METHOD_ROUTES_1 + RoutesTestConfiguration.METHOD_ROUTES_2, HttpRoute.Type.INTERNAL)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTES_2 + RoutesTestConfiguration.METHOD_ROUTES_1 + RoutesTestConfiguration.METHOD_ROUTES_2, HttpRoute.Type.INTERNAL)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTES_1 + RoutesTestConfiguration.METHOD_ROUTES_1, HttpRoute.Type.PRIVATE)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTES_1 + RoutesTestConfiguration.METHOD_ROUTES_2, HttpRoute.Type.PRIVATE)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTES_2 + RoutesTestConfiguration.METHOD_ROUTES_1, HttpRoute.Type.PRIVATE)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTES_2 + RoutesTestConfiguration.METHOD_ROUTES_2, HttpRoute.Type.PRIVATE)));
        }
    }

    @Test
    void testGetRequestMappingPaths_TestController2() {
        try (ScanResult scan = new ClassGraph()
                .enableAllInfo()
                .acceptClasses(TestController2.class.getName())
                .scan()) {

            ClassInfo info = scan.getClassInfo(TestController2.class.getName());
            Set<HttpRoute> routes = mojo.getRequestMappingPaths(info);

            assertEquals(3, routes.size());
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.METHOD_ROUTES_1, HttpRoute.Type.INTERNAL)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.METHOD_ROUTES_2, HttpRoute.Type.INTERNAL)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.METHOD_ROUTES_1 + RoutesTestConfiguration.METHOD_ROUTES_2, HttpRoute.Type.PRIVATE)));
        }
    }

    @Test
    void testGetRequestMappingPaths_TestController3() {
        try (ScanResult scan = new ClassGraph()
                .enableAllInfo()
                .acceptClasses(TestController3.class.getName())
                .scan()) {

            ClassInfo info = scan.getClassInfo(TestController3.class.getName());
            Set<HttpRoute> routes = mojo.getRequestMappingPaths(info);

            assertEquals(4, routes.size());
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTE_PATH_TO_1 + RoutesTestConfiguration.METHOD_ROUTE_PATH_TO_1, RoutesTestConfiguration.CLASS_ROUTE_PATH_FROM_1 + RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_1, HttpRoute.Type.INTERNAL)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTE_PATH_TO_1 + RoutesTestConfiguration.METHOD_ROUTE_PATH_TO_1, RoutesTestConfiguration.CLASS_ROUTE_PATH_FROM_1 + RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_2, HttpRoute.Type.INTERNAL)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTE_PATH_TO_1 + RoutesTestConfiguration.METHOD_ROUTE_PATH_TO_1, RoutesTestConfiguration.CLASS_ROUTE_PATH_FROM_2 + RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_1, HttpRoute.Type.INTERNAL)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTE_PATH_TO_1 + RoutesTestConfiguration.METHOD_ROUTE_PATH_TO_1, RoutesTestConfiguration.CLASS_ROUTE_PATH_FROM_2 + RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_2, HttpRoute.Type.INTERNAL)));
        }
    }

    @Test
    void testGetRequestMappingPaths_TestController4() {
        try (ScanResult scan = new ClassGraph()
                .enableAllInfo()
                .acceptClasses(TestController4.class.getName())
                .scan()) {

            ClassInfo info = scan.getClassInfo(TestController4.class.getName());
            Set<HttpRoute> routes = mojo.getRequestMappingPaths(info);

            assertEquals(4, routes.size());
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.METHOD_ROUTE_PATH_TO_1, RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_1, HttpRoute.Type.PUBLIC)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.METHOD_ROUTE_PATH_TO_1, RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_2, HttpRoute.Type.PUBLIC)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.METHOD_ROUTE_PATH_TO_2, RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_1 + RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_1, HttpRoute.Type.PRIVATE)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.METHOD_ROUTE_PATH_TO_2, RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_2 + RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_2, HttpRoute.Type.PRIVATE)));
        }
    }

    @Test
    void testGetRequestMappingPaths_TestController5() {
        try (ScanResult scan = new ClassGraph()
                .enableAllInfo()
                .acceptClasses(TestController5.class.getName())
                .scan()) {

            ClassInfo info = scan.getClassInfo(TestController5.class.getName());
            Set<HttpRoute> routes = mojo.getRequestMappingPaths(info);

            assertEquals(3, routes.size());
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTES_3 + RoutesTestConfiguration.METHOD_ROUTES_1, RoutesTestConfiguration.CLASS_ROUTES_3 + RoutesTestConfiguration.METHOD_ROUTES_1, HttpRoute.Type.PUBLIC)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTES_3 + RoutesTestConfiguration.METHOD_ROUTES_2, RoutesTestConfiguration.CLASS_ROUTES_3 + RoutesTestConfiguration.METHOD_ROUTES_2, HttpRoute.Type.INTERNAL)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTES_3 + RoutesTestConfiguration.METHOD_ROUTES_3, RoutesTestConfiguration.CLASS_ROUTES_3 + RoutesTestConfiguration.METHOD_ROUTES_3, HttpRoute.Type.PRIVATE)));
        }
    }

    @Test
    void testGetRequestMappingPaths_TestController6() {
        try (ScanResult scan = new ClassGraph()
                .enableAllInfo()
                .acceptClasses(TestController6.class.getName())
                .scan()) {

            ClassInfo info = scan.getClassInfo(TestController6.class.getName());
            Set<HttpRoute> routes = mojo.getRequestMappingPaths(info);

            assertEquals(1, routes.size());
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTES_4 + RoutesTestConfiguration.METHOD_ROUTES_1, "/custom" + RoutesTestConfiguration.METHOD_ROUTES_1, HttpRoute.Type.PUBLIC)));
        }
    }
}

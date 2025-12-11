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
    void testGetRequestMappingPaths_SpringTestController1() {
        try (ScanResult scan = new ClassGraph()
                .enableAllInfo()
                .acceptClasses(SpringTestController1.class.getName())
                .scan()) {

            ClassInfo info = scan.getClassInfo(SpringTestController1.class.getName());
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
    void testGetRequestMappingPaths_SpringTestController2() {
        try (ScanResult scan = new ClassGraph()
                .enableAllInfo()
                .acceptClasses(SpringTestController2.class.getName())
                .scan()) {

            ClassInfo info = scan.getClassInfo(SpringTestController2.class.getName());
            Set<HttpRoute> routes = mojo.getRequestMappingPaths(info);

            assertEquals(3, routes.size());
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.METHOD_ROUTES_1 + "/{id}", HttpRoute.Type.INTERNAL)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.METHOD_ROUTES_2, HttpRoute.Type.INTERNAL)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.METHOD_ROUTES_1 + RoutesTestConfiguration.METHOD_ROUTES_2, HttpRoute.Type.PRIVATE)));
        }
    }

    @Test
    void testGetRequestMappingPaths_SpringTestController3() {
        try (ScanResult scan = new ClassGraph()
                .enableAllInfo()
                .acceptClasses(SpringTestController3.class.getName())
                .scan()) {

            ClassInfo info = scan.getClassInfo(SpringTestController3.class.getName());
            Set<HttpRoute> routes = mojo.getRequestMappingPaths(info);

            assertEquals(4, routes.size());
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTE_PATH_TO_1 + RoutesTestConfiguration.METHOD_ROUTE_PATH_TO_1, RoutesTestConfiguration.CLASS_ROUTE_PATH_FROM_1 + RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_1, HttpRoute.Type.INTERNAL)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTE_PATH_TO_1 + RoutesTestConfiguration.METHOD_ROUTE_PATH_TO_1, RoutesTestConfiguration.CLASS_ROUTE_PATH_FROM_1 + RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_2, HttpRoute.Type.INTERNAL)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTE_PATH_TO_1 + RoutesTestConfiguration.METHOD_ROUTE_PATH_TO_1, RoutesTestConfiguration.CLASS_ROUTE_PATH_FROM_2 + RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_1, HttpRoute.Type.INTERNAL)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTE_PATH_TO_1 + RoutesTestConfiguration.METHOD_ROUTE_PATH_TO_1, RoutesTestConfiguration.CLASS_ROUTE_PATH_FROM_2 + RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_2, HttpRoute.Type.INTERNAL)));
        }
    }

    @Test
    void testGetRequestMappingPaths_SpringTestController4() {
        try (ScanResult scan = new ClassGraph()
                .enableAllInfo()
                .acceptClasses(SpringTestController4.class.getName())
                .scan()) {

            ClassInfo info = scan.getClassInfo(SpringTestController4.class.getName());
            Set<HttpRoute> routes = mojo.getRequestMappingPaths(info);

            assertEquals(4, routes.size());
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.METHOD_ROUTE_PATH_TO_1, RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_1, HttpRoute.Type.PUBLIC)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.METHOD_ROUTE_PATH_TO_1, RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_2, HttpRoute.Type.PUBLIC)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.METHOD_ROUTE_PATH_TO_2, RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_1 + RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_1, HttpRoute.Type.PRIVATE)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.METHOD_ROUTE_PATH_TO_2, RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_2 + RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_2, HttpRoute.Type.PRIVATE)));
        }
    }

    @Test
    void testGetRequestMappingPaths_SpringTestController5() {
        try (ScanResult scan = new ClassGraph()
                .enableAllInfo()
                .acceptClasses(SpringSpringTestController5.class.getName())
                .scan()) {

            ClassInfo info = scan.getClassInfo(SpringSpringTestController5.class.getName());
            Set<HttpRoute> routes = mojo.getRequestMappingPaths(info);

            assertEquals(3, routes.size());
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTES_3 + RoutesTestConfiguration.METHOD_ROUTES_1, RoutesTestConfiguration.CLASS_ROUTES_3 + RoutesTestConfiguration.METHOD_ROUTES_1, HttpRoute.Type.PUBLIC)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTES_3 + RoutesTestConfiguration.METHOD_ROUTES_2, RoutesTestConfiguration.CLASS_ROUTES_3 + RoutesTestConfiguration.METHOD_ROUTES_2, HttpRoute.Type.INTERNAL)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTES_3 + RoutesTestConfiguration.METHOD_ROUTES_3, RoutesTestConfiguration.CLASS_ROUTES_3 + RoutesTestConfiguration.METHOD_ROUTES_3, HttpRoute.Type.PRIVATE)));
        }
    }

    @Test
    void testGetRequestMappingPaths_SpringTestController6() {
        try (ScanResult scan = new ClassGraph()
                .enableAllInfo()
                .acceptClasses(SpringSpringTestController6.class.getName())
                .scan()) {

            ClassInfo info = scan.getClassInfo(SpringSpringTestController6.class.getName());
            Set<HttpRoute> routes = mojo.getRequestMappingPaths(info);

            assertEquals(1, routes.size());
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTES_4 + RoutesTestConfiguration.METHOD_ROUTES_1, "/custom" + RoutesTestConfiguration.METHOD_ROUTES_1, HttpRoute.Type.PUBLIC)));
        }
    }

    @Test
    void testGetRequestMappingPaths_QuarkusTestController1() {
        try (ScanResult scan = new ClassGraph()
                .enableAllInfo()
                .acceptClasses(QuarkusTestController1.class.getName())
                .scan()) {

            ClassInfo info = scan.getClassInfo(QuarkusTestController1.class.getName());
            Set<HttpRoute> routes = mojo.getRequestMappingPaths(info);

            assertEquals(2, routes.size());
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTES_1 + RoutesTestConfiguration.METHOD_ROUTES_1 + RoutesTestConfiguration.METHOD_ROUTES_2, HttpRoute.Type.INTERNAL)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTES_1 + RoutesTestConfiguration.METHOD_ROUTES_1, HttpRoute.Type.PRIVATE)));
        }
    }

    @Test
    void testGetRequestMappingPaths_QuarkusTestController2() {
        try (ScanResult scan = new ClassGraph()
                .enableAllInfo()
                .acceptClasses(QuarkusTestController2.class.getName())
                .scan()) {

            ClassInfo info = scan.getClassInfo(QuarkusTestController2.class.getName());
            Set<HttpRoute> routes = mojo.getRequestMappingPaths(info);

            assertEquals(2, routes.size());
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.METHOD_ROUTES_1, HttpRoute.Type.INTERNAL)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.METHOD_ROUTES_1 + RoutesTestConfiguration.METHOD_ROUTES_2, HttpRoute.Type.PRIVATE)));
        }
    }

    @Test
    void testGetRequestMappingPaths_QuarkusTestController3() {
        try (ScanResult scan = new ClassGraph()
                .enableAllInfo()
                .acceptClasses(QuarkusTestController3.class.getName())
                .scan()) {

            ClassInfo info = scan.getClassInfo(QuarkusTestController3.class.getName());
            Set<HttpRoute> routes = mojo.getRequestMappingPaths(info);

            assertEquals(2, routes.size());
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTE_PATH_TO_1 + RoutesTestConfiguration.METHOD_ROUTE_PATH_TO_1, RoutesTestConfiguration.CLASS_ROUTE_PATH_FROM_1 + RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_1, HttpRoute.Type.INTERNAL)));
            assertTrue(routes.contains(new HttpRoute(RoutesTestConfiguration.CLASS_ROUTE_PATH_TO_1 + RoutesTestConfiguration.METHOD_ROUTE_PATH_TO_1, RoutesTestConfiguration.CLASS_ROUTE_PATH_FROM_1 + RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_2, HttpRoute.Type.INTERNAL)));
        }
    }
}

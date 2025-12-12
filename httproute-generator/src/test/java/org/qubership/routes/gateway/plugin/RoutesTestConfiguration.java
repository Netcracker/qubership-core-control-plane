package org.qubership.routes.gateway.plugin;

import com.netcracker.cloud.routesregistration.common.gateway.route.RouteEntry;
import org.springframework.context.annotation.Configuration;

import java.util.List;

@Configuration
public class RoutesTestConfiguration {

    static final String CLASS_ROUTES_1 = "/api/v1/test1";
    static final String CLASS_ROUTES_2 = "/api/v1/test2";
    static final String CLASS_ROUTES_3 = "/api/v1/test3";
    static final String CLASS_ROUTES_4 = "/api/v1/test4";
    static final String CLASS_ROUTES_8 = "/api/v1/test8";
    static final String CLASS_ROUTES_10 = "/api/v1/test10";
    static final String CLASS_ROUTES_11 = "/api/v1/test11";
    static final String CLASS_ROUTES_12 = "/api/v1/test12";
    static final String METHOD_ROUTES_1 = "/test1";
    static final String METHOD_ROUTES_2 = "/test2";
    static final String METHOD_ROUTES_3 = "/test3";
    static final String CLASS_ROUTE_PATH_FROM_1 = "/class/path/from";
    static final String METHOD_ROUTE_PATH_FROM_1 = "/method/path/from";
    static final String CLASS_ROUTE_PATH_TO_1 = "/class/path/to";
    static final String METHOD_ROUTE_PATH_TO_1 = "/method/path/to";
    static final String CLASS_ROUTE_PATH_FROM_2 = "/class/path/from2";
    static final String METHOD_ROUTE_PATH_FROM_2 = "/method/path/from2";
    static final String METHOD_ROUTE_PATH_TO_2 = "/method/path/to2";

    public static final String CLOUD_MICROSERVICE_NAME = "cloud.microservice.name";
    public static final String SPRING_CLOUD_CONFIG_URI = "spring.cloud.config.uri";
    public static final String SPRING_APPLICATION_NAME_VALUE = "RoutesTestConfiguration";
    public static final String SPRING_CLOUD_CONFIG_URI_VALUE = "http:localhost:8888";

    public static final String ROUTES_REGISTRATION_URL = "/api/v2/control-plane/routes";
    public static final String CONTROL_PLANE_URL = "http://control-plane:8080";
    public static final String INTERNAL_NODE_GROUP = "internal-gateway-service";
    public static final String PUBLIC_NODE_GROUP = "public-gateway-service";
    public static final String PRIVATE_NODE_GROUP = "private-gateway-service";
    public static final String MICROSERVICE_TEST_NAME = "ms-core-test";
    public static final String PORT = "8080";
    public static final String CONTEXT_PATH = "/contextPath";
    public static final String MICROSERVICE_URL = MICROSERVICE_TEST_NAME + ":" + PORT + CONTEXT_PATH;
    public static final String INGRESS_GATEWAY = "ingress-gateway";

    public static final int CORES_NUM = Runtime.getRuntime().availableProcessors();
    public static final int THREADS_NUM = CORES_NUM;
    public static final String APIGATEWAY_ROUTES_REGISTRATION_URL = "apigateway.routes.registration.url";
    public static final String APIGATEWAY_CONTROL_PLANE_URL = "apigateway.nodegroup.private";
    public static final String SERVER_PORT = "server.port";
    public static final String SERVER_CONTEXT_PATH = "server.servlet.context-path";
    public static final String DEFAULT_INGRESS_GATEWAY = "default-ingress-gateway";
    public static final String DEFAULT_VHOST = "default-vhost";

    static List<RouteEntry> ROUTES_LIST;

    public static final long TEST_TIMEOUT_1 = 150000;
    public static final long TEST_TIMEOUT_2 = 250000;
}

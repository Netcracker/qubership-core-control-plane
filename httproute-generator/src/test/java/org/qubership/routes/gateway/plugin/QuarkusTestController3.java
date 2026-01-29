package org.qubership.routes.gateway.plugin;

import com.netcracker.cloud.routesregistration.common.annotation.Gateway;
import com.netcracker.cloud.routesregistration.common.annotation.Route;
import com.netcracker.cloud.routesregistration.common.spring.gateway.route.annotation.GatewayRequestMapping;
import jakarta.ws.rs.POST;
import jakarta.ws.rs.Path;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestMethod;
import org.springframework.web.bind.annotation.RestController;

@Route
@Gateway(RoutesTestConfiguration.CLASS_ROUTE_PATH_FROM_1)
@Path(RoutesTestConfiguration.CLASS_ROUTE_PATH_TO_1)
public class QuarkusTestController3 {

    @POST
    @Route
    @Gateway({RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_1, RoutesTestConfiguration.METHOD_ROUTE_PATH_FROM_2})
    @Path(RoutesTestConfiguration.METHOD_ROUTE_PATH_TO_1)
    public void method1() {
    }

}

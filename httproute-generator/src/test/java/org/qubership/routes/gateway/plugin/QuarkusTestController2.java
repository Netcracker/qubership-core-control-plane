package org.qubership.routes.gateway.plugin;

import com.netcracker.cloud.routesregistration.common.annotation.Route;
import com.netcracker.cloud.routesregistration.common.gateway.route.RouteType;
import jakarta.ws.rs.GET;
import jakarta.ws.rs.POST;
import jakarta.ws.rs.Path;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestMethod;
import org.springframework.web.bind.annotation.RestController;

public class QuarkusTestController2 {

    @POST
    @Path(RoutesTestConfiguration.METHOD_ROUTES_1)
    @Route
    public void method11() {
    }

    @POST
    @Path(RoutesTestConfiguration.METHOD_ROUTES_1 + RoutesTestConfiguration.METHOD_ROUTES_2)
    @Route(type = RouteType.PRIVATE)
    public void method12() {
    }

    // duplicate route
    @POST
    @Path(RoutesTestConfiguration.METHOD_ROUTES_1 + RoutesTestConfiguration.METHOD_ROUTES_2)
    @Route(RouteType.PRIVATE)
    public void method13() {
    }

}

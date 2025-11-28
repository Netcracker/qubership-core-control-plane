package org.qubership.remesh.dto;

import java.util.List;

import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
public class RouteConfig {
//    private String version; //not used
    private List<RouteV3> routes;
}

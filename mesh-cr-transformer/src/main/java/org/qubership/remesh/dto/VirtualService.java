package org.qubership.remesh.dto;

import java.util.List;

import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
public class VirtualService {
    private String name; //TODO need to check
    private List<String> hosts;
//    private String rateLimit; //not used
    private List<HeaderDefinition> addHeaders;
    private List<String> removeHeaders;
    private RouteConfig routeConfiguration;
//    private Boolean overridden; //not used
}

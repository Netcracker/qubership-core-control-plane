package org.qubership.remesh.dto;

import java.util.List;

import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
public class StatefulSession { //not used
    private String version;
    private String namespace;
    private String cluster;
    private String hostname;
    private List<String> gateways;
    private Integer port;
    private Boolean enabled;
    private Cookie cookie;
    private RouteMatcher route;
    private Boolean overridden;
}

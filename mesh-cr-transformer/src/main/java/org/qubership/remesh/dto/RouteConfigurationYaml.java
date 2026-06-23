package org.qubership.remesh.dto;

import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
public class RouteConfigurationYaml {
    private String apiVersion;
    private String kind;
    private String subKind;
    private Metadata metadata;
    private RoutingConfigRequestV3 spec;
}

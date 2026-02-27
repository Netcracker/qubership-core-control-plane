package org.qubership.remesh.dto;

import java.util.List;

import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
public class RoutingConfigRequestV3 {
//    private String namespace; //not used
    private List<String> gateways;
//    private Integer listenerPort; //not used
//    private Boolean tlsSupported; //not used
    private List<VirtualService> virtualServices;
//    private Boolean overridden; //not used
}

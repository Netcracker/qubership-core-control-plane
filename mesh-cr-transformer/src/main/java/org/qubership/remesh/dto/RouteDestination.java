package org.qubership.remesh.dto;

import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
public class RouteDestination {
//    private String cluster; //not used
//    private Boolean tlsSupported; //not used
    private String endpoint;
//    private String tlsEndpoint; //not used
//    private Integer httpVersion; //not used
//    private String tlsConfigName; //not used
//    private CircuitBreaker circuitBreaker; //not used
//    private TcpKeepalive tcpKeepalive; //not used
}

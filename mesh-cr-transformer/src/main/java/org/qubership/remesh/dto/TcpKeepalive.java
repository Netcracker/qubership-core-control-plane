package org.qubership.remesh.dto;

import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
public class TcpKeepalive { //not used
    private Integer probes;
    private Integer time;
    private Integer interval;
}

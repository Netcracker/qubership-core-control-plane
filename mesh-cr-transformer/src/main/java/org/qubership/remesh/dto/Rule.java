package org.qubership.remesh.dto;

import java.util.List;

import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
public class Rule {
    private RouteMatch match;
    private String prefixRewrite;
    private String hostRewrite;
    private List<HeaderDefinition> addHeaders;
    private List<String> removeHeaders;
    private Boolean allowed; //TODO need to check
//    private Long timeout; //not used
//    private Long idleTimeout; //not used
//    private StatefulSession statefulSession; //not used
//    private String rateLimit; //not used
//    private Boolean deny; //not used
//    private String luaFilter; //not used
}

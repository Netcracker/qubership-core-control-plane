package org.qubership.remesh.dto;

import java.util.List;

import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
public class RouteV3 {
    private RouteDestination destination;
    private List<Rule> rules;
}

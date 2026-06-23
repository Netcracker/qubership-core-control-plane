package org.qubership.remesh.dto;

import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.Map;

@Data
@NoArgsConstructor
public class Metadata {
    private String name;
    private String namespace;
    private String gateway;
    private Map<String, String> labels;
}

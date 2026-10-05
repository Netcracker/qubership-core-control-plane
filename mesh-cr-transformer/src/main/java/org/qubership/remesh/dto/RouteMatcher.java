package org.qubership.remesh.dto;

import java.util.List;

import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
public class RouteMatcher { //not used
    private String prefix;
    private String regExp;
    private String path;
    private List<HeaderMatcher> headerMatchers;
    private List<HeaderDefinition> addHeaders;
    private List<String> removeHeaders;
}

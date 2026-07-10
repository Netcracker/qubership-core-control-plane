package org.qubership.remesh.dto;

import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
public class HeaderMatcher {
    private String name;
    private String exactMatch;
    private String safeRegexMatch;
//    RangeMatch     rangeMatch; //not used
    private boolean presentMatch;
    private String prefixMatch;
    private String suffixMatch;
//    private boolean invertMatch; //not used
}

package org.qubership.remesh.util;

import java.util.regex.Matcher;
import java.util.regex.Pattern;

public class EndpointParser {
    // scheme://host:port
    private static final Pattern ENDPOINT_PATTERN = Pattern.compile(
            "^(?<scheme>[a-zA-Z][a-zA-Z0-9+.-]*)://" +
            "(?<host>[^:/]+)" +
            "(?::(?<port>[^/]+))?.*$"
    );

    public static EndpointDTO parse(String endpoint) {
        if (endpoint == null || endpoint.isBlank()) {
            throw new IllegalArgumentException("Endpoint cannot be null or blank");
        }

        Matcher matcher = ENDPOINT_PATTERN.matcher(endpoint);
        if (!matcher.matches()) {
            throw new IllegalArgumentException("Invalid endpoint format: " + endpoint);
        }

        String scheme = matcher.group("scheme");
        String host = matcher.group("host");
        String port = matcher.group("port");

        return new EndpointDTO(scheme, host, port);
    }
}

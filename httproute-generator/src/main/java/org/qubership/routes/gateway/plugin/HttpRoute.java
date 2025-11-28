package org.qubership.routes.gateway.plugin;

public record HttpRoute(String path, Type type) {
    public enum Type {
        FACADE,
        INTERNAL,
        PRIVATE,
        PUBLIC
    }
}

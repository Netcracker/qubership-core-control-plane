package org.qubership.routes.gateway.plugin;

public class HttpRoute {
    private final String path;
    private final RouteType type;

    public HttpRoute(String path, RouteType type) {
        this.path = path;
        this.type = type;
    }

    public String getPath() {
        return path;
    }

    public RouteType getType() {
        return type;
    }

    @Override
    public String toString() {
        return "HttpRoute{" +
                "path='" + path + '\'' +
                ", type=" + type +
                '}';
    }
}

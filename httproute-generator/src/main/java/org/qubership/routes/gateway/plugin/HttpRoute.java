package org.qubership.routes.gateway.plugin;

public record HttpRoute(String path, String gatewayPath, Type type, long timeout) {

    public HttpRoute(String path, Type type, long timeout) {
        this(path, path, type, timeout);
    }

    public HttpRoute(String path, Type type) {
        this(path, path, type);
    }

    public HttpRoute(String path, String gatewayPath, Type type) {
        this(path, gatewayPath, type, 0);
    }

    public enum Type {
        FACADE("facade-gateway-istio"),
        INTERNAL("internal-gateway-istio"),
        PRIVATE("private-gateway-istio"),
        PUBLIC("public-gateway-istio");

        private final String gatewayName;

        public String gatewayName() {
            return gatewayName;
        }

        Type(String gatewayName) {
            this.gatewayName = gatewayName;
        }
    }
}

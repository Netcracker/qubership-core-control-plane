package org.qubership.routes.gateway.plugin;

public record HttpRoute(String path, Type type) {
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

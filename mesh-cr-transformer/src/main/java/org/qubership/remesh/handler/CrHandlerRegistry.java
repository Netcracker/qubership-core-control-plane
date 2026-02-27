package org.qubership.remesh.handler;

import java.util.Collections;
import java.util.HashMap;
import java.util.Map;
import java.util.ServiceLoader;

public final class CrHandlerRegistry {
    private static final Map<String, CrHandler> HANDLERS = loadHandlers();

    public static CrHandler get(String kind) {
        return HANDLERS.get(kind);
    }

    private static Map<String, CrHandler> loadHandlers() {
        Map<String, CrHandler> result = new HashMap<>();

        ServiceLoader<CrHandler> loader = ServiceLoader.load(CrHandler.class);

        for (CrHandler handler : loader) {
            String kind = handler.getKind();

            if (result.containsKey(kind)) {
                throw new IllegalStateException(
                        "Duplicate CrHandler for kind: " + kind +
                        " (" + handler.getClass().getName() + ")"
                );
            }

            result.put(kind, handler);
        }

        return Collections.unmodifiableMap(result);
    }

    private CrHandlerRegistry() {}
}

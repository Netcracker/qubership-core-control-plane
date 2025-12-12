package org.qubership.remesh.handler;

import org.junit.jupiter.api.Test;

import java.lang.reflect.Field;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

class CrHandlerRegistryTest {

    @Test
    void returnsRegisteredHandler() {
        CrHandler handler = CrHandlerRegistry.get("RouteConfiguration");

        assertNotNull(handler);
        assertEquals(RouteConfigurationHandler.class, handler.getClass());
    }

    @Test
    void handlersMapIsUnmodifiable() throws Exception {
        Field field = CrHandlerRegistry.class.getDeclaredField("HANDLERS");
        field.setAccessible(true);
        Map<String, CrHandler> handlers = (Map<String, CrHandler>) field.get(null);

        assertThrows(UnsupportedOperationException.class, () -> handlers.put("test", new DummyHandler()));
    }

    private static class DummyHandler implements CrHandler {
        @Override
        public String getKind() {
            return "test";
        }

        @Override
        public java.util.List<Resource> handle(com.fasterxml.jackson.databind.JsonNode node) {
            return java.util.List.of();
        }
    }
}

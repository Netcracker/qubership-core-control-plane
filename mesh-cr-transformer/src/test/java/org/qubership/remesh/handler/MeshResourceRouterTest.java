package org.qubership.remesh.handler;

import com.fasterxml.jackson.databind.JsonNode;
import org.junit.jupiter.api.Test;
import org.qubership.remesh.util.ObjectMapperProvider;

import java.util.List;
import java.util.concurrent.atomic.AtomicBoolean;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

class MeshResourceRouterTest {

    @Test
    void routesMeshResourceToHandler() throws Exception {
        JsonNode node = ObjectMapperProvider.getMapper().readTree("""
                apiVersion: core.netcracker.com/v1
                kind: Mesh
                subKind: Demo
                spec:
                  foo: bar
                """);

        MeshResourceFragment fragment = MeshResourceFragment.create(0, node, null);

        AtomicBoolean handled = new AtomicBoolean(false);
        MeshResourceRouter router = new MeshResourceRouter(kind -> new CrHandler() {
            @Override
            public String getKind() {
                return "Demo";
            }

            @Override
            public List<Resource> handle(MeshResourceFragment ignored) {
                handled.set(true);
                return List.of(new TestResource());
            }
        });

        List<Resource> result = router.route(fragment);

        assertTrue(handled.get());
        assertEquals(1, result.size());
    }

    @Test
    void skipsNonMeshResource() throws Exception {
        JsonNode node = ObjectMapperProvider.getMapper().readTree("""
                apiVersion: demo/v1
                kind: Something
                subKind: Demo
                spec:
                  foo: bar
                """);

        MeshResourceFragment fragment = MeshResourceFragment.create(0, node, null);
        MeshResourceRouter router = new MeshResourceRouter(kind -> null);

        List<Resource> result = router.route(fragment);

        assertTrue(result.isEmpty());
    }

    private static class TestResource implements Resource {
        @Override
        public String getApiVersion() {
            return "demo/v1";
        }

        @Override
        public String getKind() {
            return "Demo";
        }

        @Override
        public String getRawMetadata() {
            return null;
        }

        @Override
        public void setRawMetadata(String rawMetadata) {
        }
    }
}

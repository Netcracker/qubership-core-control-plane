package org.qubership.remesh.validation;

import org.junit.jupiter.api.Test;
import org.qubership.remesh.handler.Resource;

import static org.junit.jupiter.api.Assertions.assertEquals;

class ResourceValidatorTest {

    @Test
    void buildsSchemaFileName() {
        ResourceValidator validator = new ResourceValidator();

        String name = validator.schemaFileName(new Resource() {
            @Override
            public String getApiVersion() {
                return "gateway.networking.k8s.io/v1";
            }

            @Override
            public String getKind() {
                return "HTTPRoute";
            }
        });

        assertEquals("gateway.networking.k8s.io_httproute.yaml", name);
    }
}

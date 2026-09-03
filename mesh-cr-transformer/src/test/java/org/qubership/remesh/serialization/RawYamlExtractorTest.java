package org.qubership.remesh.serialization;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

class RawYamlExtractorTest {

    private final RawYamlExtractor extractor = new RawYamlExtractor();

    @Test
    void extractsMetadataWithHelmTemplates() {
        String yaml = """
                apiVersion: core.netcracker.com/v1
                kind: Mesh
                subKind: RouteConfiguration
                metadata:
                  # is not equal to {{ .Values.SERVICE_NAME }}-mesh-routes
                  name: account-mgmt-billing-integration-mesh-routes
                  namespace: "{{ .Values.NAMESPACE }}"
                  labels:
                    {{ include "cloudbss-lib.labels.common" . | nindent 4 | trim }}
                    deployer.cleanup/allow: "true"
                    app.kubernetes.io/instance: '{{ cat (coalesce .Values.DEPLOYMENT_RESOURCE_NAME .Values.SERVICE_NAME) "-" .Values.NAMESPACE | nospace | trunc 63 | trimSuffix "-" }}'
                    app.kubernetes.io/processed-by-operator: 'core-operator'
                spec:
                  gateways: ["cbm-composite-gateway"]
                """;

        String metadata = extractor.extractMetadata(yaml);

        assertNotNull(metadata);
        assertTrue(metadata.startsWith("metadata:"));
        assertTrue(metadata.contains("# is not equal to {{ .Values.SERVICE_NAME }}-mesh-routes"));
        assertTrue(metadata.contains("{{ include \"cloudbss-lib.labels.common\" . | nindent 4 | trim }}"));
        assertTrue(metadata.contains("'{{ cat (coalesce .Values.DEPLOYMENT_RESOURCE_NAME .Values.SERVICE_NAME) \"-\" .Values.NAMESPACE | nospace | trunc 63 | trimSuffix \"-\" }}'"));
        assertFalse(metadata.contains("spec:"));
        assertFalse(metadata.contains("gateways:"));
    }

    @Test
    void extractsSimpleMetadata() {
        String yaml = """
                apiVersion: v1
                kind: ConfigMap
                metadata:
                  name: my-config
                  namespace: default
                spec:
                  key: value
                """;

        System.out.println(yaml);

        String metadata = extractor.extractMetadata(yaml);

        System.out.println(metadata);

        assertNotNull(metadata);
        assertTrue(metadata.contains("name: my-config"));
        assertTrue(metadata.contains("namespace: default"));
        assertFalse(metadata.contains("spec:"));
    }

    @Test
    void returnsNullWhenNoMetadata() {
        String yaml = """
                apiVersion: v1
                kind: ConfigMap
                data:
                  key: value
                """;

        String metadata = extractor.extractMetadata(yaml);

        assertNull(metadata);
    }

    @Test
    void preservesComments() {
        String yaml = """
                apiVersion: v1
                kind: Test
                metadata:
                  # This is a comment
                  name: test
                  # Another comment
                  namespace: default
                spec:
                  field: value
                """;

        String metadata = extractor.extractMetadata(yaml);

        assertNotNull(metadata);
        assertTrue(metadata.contains("# This is a comment"));
        assertTrue(metadata.contains("# Another comment"));
    }

    @Test
    void extractsSpecSection() {
        String yaml = """
                apiVersion: v1
                kind: Test
                metadata:
                  name: test
                spec:
                  field1: value1
                  field2: value2
                status:
                  ready: true
                """;

        String spec = extractor.extractSection(yaml, "spec");

        assertNotNull(spec);
        assertTrue(spec.startsWith("spec:"));
        assertTrue(spec.contains("field1: value1"));
        assertTrue(spec.contains("field2: value2"));
        assertFalse(spec.contains("status:"));
    }

    @Test
    void handlesMetadataAtEndOfDocument() {
        String yaml = """
                apiVersion: v1
                kind: Test
                metadata:
                  name: test
                  namespace: default
                """;

        String metadata = extractor.extractMetadata(yaml);

        assertNotNull(metadata);
        assertTrue(metadata.contains("name: test"));
        assertTrue(metadata.contains("namespace: default"));
    }
}

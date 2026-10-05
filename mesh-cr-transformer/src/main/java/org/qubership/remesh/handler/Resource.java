package org.qubership.remesh.handler;

public interface Resource {
    String getApiVersion();
    String getKind();

    /**
     * Returns the raw metadata section as YAML string,
     * preserving comments, Helm templates, and original formatting.
     * May return null if raw metadata is not available.
     */
    String getRawMetadata();

    /**
     * Sets the raw metadata section.
     */
    void setRawMetadata(String rawMetadata);

    default String getFullVersion() {
        return getApiVersion() + ":" + getKind();
    }
}

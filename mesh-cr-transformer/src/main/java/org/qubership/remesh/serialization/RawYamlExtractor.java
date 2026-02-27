package org.qubership.remesh.serialization;

/**
 * Utility class for extracting YAML sections as raw strings,
 * preserving comments, Helm templates, and original formatting.
 */
public class RawYamlExtractor {

    /**
     * Extracts a top-level section from YAML as raw string.
     * Preserves comments, Helm templates, and original formatting.
     *
     * @param yaml        the full YAML document
     * @param sectionName the section name to extract (e.g., "metadata", "spec")
     * @return the raw section content including the section header, or null if not found
     */
    public String extractSection(String yaml, String sectionName) {
        String[] lines = yaml.split("\n", -1);
        String sectionStart = sectionName + ":";

        StringBuilder result = new StringBuilder();
        boolean inSection = false;

        for (String line : lines) {
            String trimmed = line.trim();

            if (!inSection) {
                if (trimmed.startsWith(sectionStart)) {
                    inSection = true;
                    result.append(line);
                }
            } else {
                // New top-level key = end of section
                if (!trimmed.isEmpty() && !Character.isWhitespace(line.charAt(0)) && trimmed.contains(":")) {
                    break;
                }
                result.append("\n").append(line);
            }
        }

        if (!inSection) {
            return null;
        }

        // Trim trailing empty lines
        String res = result.toString();
        while (res.endsWith("\n") || res.endsWith("\n ")) {
            res = res.substring(0, res.lastIndexOf('\n'));
        }

        return res;
    }

    /**
     * Extracts metadata section with all its content preserved.
     *
     * @param yaml the full YAML document
     * @return the raw metadata section or null if not found
     */
    public String extractMetadata(String yaml) {
        return extractSection(yaml, "metadata");
    }
}

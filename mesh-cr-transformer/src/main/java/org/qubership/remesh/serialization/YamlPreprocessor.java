package org.qubership.remesh.serialization;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.extern.slf4j.Slf4j;

import java.util.regex.Matcher;
import java.util.regex.Pattern;

@Slf4j
public class YamlPreprocessor {
    private final ObjectMapper mapper;

    public YamlPreprocessor(ObjectMapper mapper) {
        this.mapper = mapper;
    }

    public JsonNode readAsJsonNode(String yamlFragment) {
        String preprocessed = preprocessYaml(yamlFragment);

        try {
            return mapper.readTree(preprocessed);
        } catch (Exception e) {
            log.warn("    Failed to parse document, skipping. Cause: {}", e.getMessage());
            return null;
        }
    }

    public String preprocessYaml(String yaml) {
        yaml = replaceStandaloneTemplates(yaml);
        yaml = quoteInlineTemplates(yaml);
        return yaml;
    }

    public String replaceStandaloneTemplates(String yaml) {
        return yaml.replaceAll(
                "(?m)^(\\s*)(\\{\\{[^\\n]*}})\\s*$",
                "$1# $2"
        );
    }

    public String quoteInlineTemplates(String yaml) {
        Pattern p = Pattern.compile(
                "(?m)^([ \\t]*[^:#\\n]+:)[ \\t]*" +
                        "(?![ \\t]*['\"])"+
                        "([^\\n]*\\{\\{[^\\n]+}}[^\\n]*)$"
        );

        Matcher m = p.matcher(yaml);
        StringBuilder sb = new StringBuilder();

        while (m.find()) {
            String keyPart = m.group(1);
            String valuePart = m.group(2).trim();

            if (valuePart.indexOf('[') >= 0 || valuePart.indexOf(']') >= 0) {
                m.appendReplacement(sb, Matcher.quoteReplacement(m.group(0)));
                continue;
            }

            String quotedValue = "'" + valuePart.replace("'", "''") + "'";

            m.appendReplacement(sb, Matcher.quoteReplacement(keyPart + " " + quotedValue));
        }

        m.appendTail(sb);
        return sb.toString();
    }
}

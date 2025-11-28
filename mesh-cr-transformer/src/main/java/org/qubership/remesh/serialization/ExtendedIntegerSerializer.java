package org.qubership.remesh.serialization;

import com.fasterxml.jackson.core.JsonGenerator;
import com.fasterxml.jackson.databind.JsonSerializer;
import com.fasterxml.jackson.databind.SerializerProvider;

import java.io.IOException;

public class ExtendedIntegerSerializer extends JsonSerializer<String> {

    @Override
    public void serialize(String value,
                          JsonGenerator gen,
                          SerializerProvider serializers) throws IOException {

        if (value == null) {
            gen.writeNull();
            return;
        }

        if (isHelmPlaceholder(value)) {
            gen.writeRawValue(value);
        } else if (isInteger(value)) {
            gen.writeNumber(value);
        } else {
            gen.writeString(value);
        }
    }

    boolean isHelmPlaceholder(String s) {
        String trim = s.trim();
        return trim.startsWith("{{") && trim.endsWith("}}");
    }

    boolean isInteger(String s) {
        try {
            Integer.parseInt(s);
            return true;
        } catch (NumberFormatException ex) {
            return false;
        }
    }
}

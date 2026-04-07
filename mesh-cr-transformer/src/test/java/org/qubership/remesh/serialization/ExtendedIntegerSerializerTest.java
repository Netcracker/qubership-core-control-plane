package org.qubership.remesh.serialization;

import com.fasterxml.jackson.core.JsonGenerator;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;

import java.io.StringWriter;

import static org.junit.jupiter.api.Assertions.assertEquals;

class ExtendedIntegerSerializerTest {

    private final ExtendedIntegerSerializer serializer = new ExtendedIntegerSerializer();
    private final ObjectMapper mapper = new ObjectMapper();

    @Test
    void writesNullValue() throws Exception {
        StringWriter writer = new StringWriter();
        JsonGenerator generator = mapper.getFactory().createGenerator(writer);

        serializer.serialize(null, generator, null);
        generator.flush();

        assertEquals("null", writer.toString());
    }

    @Test
    void writesHelmPlaceholderRaw() throws Exception {
        StringWriter writer = new StringWriter();
        JsonGenerator generator = mapper.getFactory().createGenerator(writer);

        serializer.serialize("{{ .Values.port }}", generator, null);
        generator.flush();

        assertEquals("{{ .Values.port }}", writer.toString());
    }

    @Test
    void writesIntegerAsNumber() throws Exception {
        StringWriter writer = new StringWriter();
        JsonGenerator generator = mapper.getFactory().createGenerator(writer);

        serializer.serialize("42", generator, null);
        generator.flush();

        assertEquals("42", writer.toString());
    }

    @Test
    void writesNonIntegerAsString() throws Exception {
        StringWriter writer = new StringWriter();
        JsonGenerator generator = mapper.getFactory().createGenerator(writer);

        serializer.serialize("abc", generator, null);
        generator.flush();

        assertEquals("\"abc\"", writer.toString());
    }
}

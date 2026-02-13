package org.qubership.remesh.util;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.core.JsonStreamContext;
import com.fasterxml.jackson.databind.DeserializationContext;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.JsonDeserializer;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.deser.DeserializationProblemHandler;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;
import com.fasterxml.jackson.dataformat.yaml.YAMLGenerator;
import lombok.extern.slf4j.Slf4j;

import java.io.IOException;
import java.util.ArrayList;
import java.util.List;

@Slf4j
public class ObjectMapperProvider {
    private static final ObjectMapper MAPPER = constructMapper();

    public static ObjectMapper getMapper() {
        return MAPPER;
    }

    static ObjectMapper constructMapper() {
        YAMLFactory yamlFactory = YAMLFactory.builder()
                .disable(YAMLGenerator.Feature.SPLIT_LINES)
                .enable(YAMLGenerator.Feature.MINIMIZE_QUOTES)
                .build();
        ObjectMapper result = new ObjectMapper(yamlFactory);

        result.configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false);

        result.addHandler(new DeserializationProblemHandler() {
            @Override
            public boolean handleUnknownProperty(DeserializationContext ctxt, JsonParser p, JsonDeserializer<?> deserializer, Object beanOrClass, String propertyName) throws IOException {
                Class<?> targetClass = (beanOrClass instanceof Class)
                        ? (Class<?>) beanOrClass
                        : beanOrClass.getClass();

                String path = buildPath(p, propertyName);

                log.warn("       Unsupported YAML property '{}' for type '{}', location: {}. Manual migration is needed",
                        propertyName,
                        targetClass.getSimpleName(),
                        path);

                p.skipChildren();
                return true;
            }
        });

        result.setSerializationInclusion(JsonInclude.Include.NON_EMPTY);

        return result;
    }

    static String buildPath(JsonParser p, String leafProperty) {
        List<Object> segments = getPathSegments(p);

        StringBuilder sb = new StringBuilder();
        for (Object seg : segments) {
            if (seg instanceof String) {
                if (!sb.isEmpty()) {
                    sb.append('.');
                }
                sb.append(seg);
            } else {
                sb.append('[').append(seg).append(']');
            }
        }

        if (!sb.isEmpty()) {
            sb.append('.').append(leafProperty);
        } else {
            sb.append(leafProperty);
        }

        return sb.toString();
    }

    static List<Object> getPathSegments(JsonParser p) {
        List<Object> segments = new ArrayList<>();

        JsonStreamContext ctx = p.getParsingContext();
        ctx = ctx.getParent();

        while (ctx != null && ctx.getParent() != null) {
            if (ctx.inArray()) {
                segments.addFirst(ctx.getCurrentIndex());
            } else if (ctx.inObject()) {
                String name = ctx.getCurrentName();
                if (name != null) {
                    segments.addFirst(name);
                }
            }
            ctx = ctx.getParent();
        }
        return segments;
    }

    private ObjectMapperProvider() {
    }
}

/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import com.fasterxml.jackson.core.JsonGenerator;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.nuclio.Event;

import com.fasterxml.jackson.annotation.JsonProperty;

import java.io.IOException;
import java.util.Date;
import java.util.Map;
import java.util.Base64;

public class JsonEvent implements Event {
    private static ObjectMapper mapper;

    private byte[] body;

    @JsonProperty("content-type") private String contentType;

    @JsonProperty("headers")
    private Map<String, Object> headers;

    @JsonProperty("fields")
    private Map<String, Object> fields;

    @JsonProperty("size")
    private long size;

    @JsonProperty("id")
    private String id;

    @JsonProperty("method")
    private String method;

    @JsonProperty("path")
    private String path;

    @JsonProperty("url")
    private String url;

    @JsonProperty("version")
    private long version;

    private Date timestamp;

    @JsonProperty("trigger")
    private Trigger trigger;

    static {
        mapper = new ObjectMapper();
    }

    public static Event decodeEvent(byte[] data) throws IOException {
        return mapper.readValue(data, JsonEvent.class);

    }

    @JsonProperty("body")
    public void setBody(String body) {
        this.body = Base64.getDecoder().decode(body);
    }

    @JsonProperty("timestamp")
    public void setTimestamp(long timestamp) {
        this.timestamp = new Date(timestamp * 1000);
    }

    // io.nuclio.Event interface
    @Override
    public byte[] getBody() {
        return this.body;
    }

    @Override
    public String getContentType() {
        return this.contentType;
    }

    @Override
    public long getVersion() {
        return this.version;
    }

    @Override
    public String getID() {
        return null;
    }

    @Override
    public String getSourceClass() {
        return this.trigger.getClassName();
    }

    @Override
    public String getSourceKind() {
        return this.trigger.getKindName();
    }

    @Override
    public Map<String, Object> getHeaders() {
        return this.headers;
    }

    @Override
    public Object getHeader(String key) {
        return this.headers.get(key);
    }

    @Override
    public String getHeaderString(String key) {
        try {
            return (String) this.getHeader(key);
        } catch (ClassCastException e) {
            return null;
        }
    }

    @Override
    public byte[] getHeaderBytes(String key) {
        String header = this.getHeaderString(key);
        if (header == null) {
            return null;
        }
        return header.getBytes();
    }

    @Override
    public Object getField(String key) {
        return this.fields.get(key);
    }

    @Override
    public String getFieldString(String key) {
        try {
            return (String) this.getField(key);
        } catch (ClassCastException err) {
            return null;
        }
    }

    @Override
    public byte[] getFieldBytes(String key) {
        String value = this.getFieldString(key);
        if (value == null) {
            return null;
        }

        return value.getBytes();
    }

    @Override
    public long getFieldLong(String key) {
        try {
            return (Long) this.getField(key);
        } catch (ClassCastException err) {
            return 0;
        }
    }

    @Override
    public Map<String, Object> getFields() {
        return this.fields;
    }

    @Override
    public Date getTimestamp() {
        return this.timestamp;
    }

    @Override
    public String getPath() {
        return this.path;
    }

    @Override
    public String getURL() {
        return this.url;
    }

    @Override
    public String getMethod() {
        return this.method;
    }

    @Override
    public long getSize() {
        return this.size;
    }
}

class Trigger {
    private String className;
    private String kindName;

    @JsonProperty("class")
    public void setClass(String value) {
        this.className = value;
    }

    public String getClassName() {
        return this.className;
    }

    @JsonProperty("kind")
    public void setKind(String value) {
        this.kindName = value;
    }

    public String getKindName() {
        return this.kindName;
    }
}

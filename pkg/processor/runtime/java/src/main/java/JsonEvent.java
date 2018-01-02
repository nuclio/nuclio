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
    private String contentType;
    private Map<String, Object> headers;
    private Map<String, Object> fields;
    private long size;
    private String id;
    private String method;
    private String path;
    private String url;
    private long version;
    private Date timestamp;
    private Trigger trigger;

    static {
        mapper = new ObjectMapper();
        mapper.configure(JsonGenerator.Feature.AUTO_CLOSE_TARGET, false);
    }

    public static Event decodeEvent(String data) throws IOException {
        return mapper.readValue(data, JsonEvent.class);

    }

    @JsonProperty("body")
    public void setBody(String body) {
        this.body = Base64.getDecoder().decode(body);
    }

    @Override
    public byte[] getBody() {
        return this.body;
    }

    @JsonProperty("content-type")
    void setContentType(String contentType) {
        this.contentType = contentType;
    }

    @Override
    public String getContentType() {
        return this.contentType;
    }

    @JsonProperty("version")
    public void setVersion(long Version) {
        this.version = version;
    }

    @Override
    public long getVersion() {
        return this.version;
    }

    @JsonProperty("id")
    public void setId(String id) {
        this.id = id;
    }

    @Override
    public String getID() {
        return null;
    }

    @JsonProperty("trigger")
    public void setTrigger(Trigger trigger) {
        this.trigger = trigger;
    }

    @Override
    public String getSourceClass() {
        return this.trigger.getClassName();
    }

    @Override
    public String getSourceKind() {
        return this.trigger.getKindName();
    }

    @JsonProperty("headers")
    public void setHeaders(Map<String, Object> headers) {
        this.headers = headers;
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

    @JsonProperty("fields")
    public void setFields(Map<String, Object> fields) {
        this.fields = fields;
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

    @JsonProperty("timestamp")
    public void setTimestamp(long timestamp) {
        this.timestamp = new Date(timestamp * 1000);
    }

    @Override
    public Date getTimestamp() {
        return this.timestamp;
    }

    @JsonProperty("path")
    public void setPath(String path) {
        this.path = path;
    }

    @Override
    public String getPath() {
        return this.path;
    }

    @JsonProperty("url")
    public void setUrl(String url) {
        this.url = url;
    }

    @Override
    public String getURL() {
        return this.url;
    }

    @JsonProperty("method")
    public void setMethod(String method) {
        this.method = method;
    }

    @Override
    public String getMethod() {
        return this.method;
    }

    @JsonProperty("size")
    public void setSize(int size) {
        this.size = size;
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

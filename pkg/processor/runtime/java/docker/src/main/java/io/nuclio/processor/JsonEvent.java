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

package io.nuclio.processor;

import java.util.Date;
import java.util.Map;

import com.google.gson.annotations.SerializedName;
import io.nuclio.TriggerInfo;


public class JsonEvent implements io.nuclio.Event {
    private byte[] body;
    @SerializedName("content-type")
    private String contentType;
    private Map<String, Object> headers;
    private Map<String, Object> fields;
    private String id;
    private String method;
    private String path;
    private String url;
    private Date timestamp;
    private Trigger trigger;
    private long shard_id;
    private long num_shards;
    private String type;
    private String type_version;
    private String version;

    @Override
    public byte[] getBody() {
        return this.body;
    }

    @Override
    public Object getBodyObject() { return this.body; }

    @Override
    public String getContentType() {
        return this.contentType;
    }

    @Override
    public String getID() {
        return this.id;
    }

    @Override
    public TriggerInfo getTriggerInfo() {
        return this.trigger;
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
    public long getHeaderLong(String key) {
        try {
            return (long) this.getHeader(key);
        } catch (ClassCastException e) {
            return 0;
        }
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
    public long getShardID() {
        return this.shard_id;
    }

    @Override
    public long getTotalNumShards() {
        return this.num_shards;
    }

    @Override
    public String getType() {
        return this.type;
    }

    @Override
    public String getTypeVersion() {
        return this.type_version;
    }

    @Override
    public String getVersion() {
        return this.version;
    }
}

class Trigger implements TriggerInfo {
    @SerializedName("class")
    String className;
    @SerializedName("kind")
    String kindName;

    public String getClassName() {
        return this.className;
    }

    public String getKindName() {
        return this.kindName;
    }
}

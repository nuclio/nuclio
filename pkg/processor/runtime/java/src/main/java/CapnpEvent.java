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

import io.nuclio.wrapper.NuclioIPC;
import io.nuclio.Event;

import java.util.Map;
import java.util.HashMap;
import java.util.Date;

import org.capnproto.StructList;

public class CapnpEvent implements io.nuclio.Event {
    private NuclioIPC.Event.Reader eventReader;
    private Date timestamp;
    private Map<String, Object> headers;
    private Map<String, Object> fields;

    private CapnpEvent(NuclioIPC.Event.Reader eventReader) {
        this.eventReader = eventReader;
    }

    private Map<String, Object> readHeaders() {
        Map<String, Object> headers = new HashMap<String, Object>();
        StructList.Reader<NuclioIPC.Entry.Reader> reader = this.eventReader.getHeaders();

        for (int i = 0; i < reader.size(); i++) {
            NuclioIPC.Entry.Reader item = reader.get(i);
            String key = item.getKey().toString();
            NuclioIPC.Entry.Value.Reader valReader = item.getValue();
            switch (valReader.which()) {
                case S_VAL:
                    headers.put(key, valReader.getSVal().toString());
                    break;
                case D_VAL:
                    headers.put(key, valReader.getDVal().toArray());
                    break;
            }
        }

        return headers;
    }

    private Map<String, Object> readFields() {
        Map<String, Object> fields = new HashMap<String, Object>();
        StructList.Reader<NuclioIPC.Entry.Reader> reader = this.eventReader.getFields();

        for (int i = 0; i < reader.size(); i++) {
            NuclioIPC.Entry.Reader item = reader.get(i);
            String key = item.getKey().toString();
            NuclioIPC.Entry.Value.Reader valReader = item.getValue();
            switch (valReader.which()) {
                case S_VAL:
                    fields.put(key, valReader.getSVal().toString());
                    break;
                case I_VAL:
                    fields.put(key, valReader.getIVal());
                    break;
                case D_VAL:
                    fields.put(key, valReader.getDVal().toArray());
                    break;
            }
        }

        return fields;
    }

    public static Event fromReader(NuclioIPC.Event.Reader eventReader) {
        return new CapnpEvent(eventReader);
    }

    public long getVersion() {
        return this.eventReader.getVersion();
    }

    public String getID() {
        return this.eventReader.getId().toString();
    }

    public String getSourceClass() {
        return this.eventReader.getSource().getClassName().toString();
    }

    public String getSourceKind() {
        return this.eventReader.getSource().getKindName().toString();
    }

    public String getContentType() {
        return this.eventReader.getContentType().toString();
    }

    public byte[] getBody() {
        return eventReader.getBody().toArray();
    }

    public long getSize() {
        return eventReader.getSize();
    }

    public Date getTimestamp() {
        if (this.timestamp == null) {
            this.timestamp = new Date(this.eventReader.getTimestamp());
        }
        return this.timestamp;
    }

    public String getPath() {
        return this.eventReader.getPath().toString();
    }

    public String getURL() {
        return this.eventReader.getUrl().toString();
    }

    public String getMethod() {
        return this.eventReader.getMethod().toString();
    }

    public Map<String, Object> getHeaders() {
        if (this.headers == null) {
            this.headers = readHeaders();
        }

        return this.headers;
    }

    public Object getHeader(String key) {
        return this.getHeaders().get(key);
    }

    public String getHeaderString(String key) {
        Object value = this.getHeaders().get(key);
        if ((value == null) || !(value instanceof String)) {
            return null;
        }

        return (String) value;
    }

    public byte[] getHeaderBytes(String key) {
        Object value = this.getHeaders().get(key);
        if ((value == null) || !(value instanceof byte[])) {
            return null;
        }

        return (byte[]) value;
    }

    public Map<String, Object> getFields() {
        if (this.fields == null) {
            this.fields = readFields();
        }

        return this.fields;
    }

    public Object getField(String key) {
        return this.getFields().get(key);
    }

    public String getFieldString(String key) {
        Object value = this.getFields().get(key);
        if ((value == null) || !(value instanceof String)) {
            return null;
        }

        return (String) value;
    }

    public byte[] getFieldBytes(String key) {
        Object value = this.getFields().get(key);
        if ((value == null) || !(value instanceof byte[])) {
            return null;
        }

        return (byte[]) value;
    }

    public long getFieldLong(String key) {
        Object value = this.getFields().get(key);
        if ((value == null) || !(value instanceof Long)) {
            return 0;
        }

        return (long) value;
    }
}

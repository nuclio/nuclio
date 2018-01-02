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

import org.capnproto.StructList;

import java.util.Set;
import java.util.HashSet;
import java.util.Map;

public class CapnpUtils {
    /**
     * Encode Set of entries to a builder according to type
     *
     * @param builder  capnp list builder
     * @param entrySet Entries to encode
     * @throws IllegalMonitorStateException
     */
    public static void encodeEntrySet(
            StructList.Builder<NuclioIPC.Entry.Builder> builder,
            Set<Map.Entry<String, Object>> entrySet) throws IllegalMonitorStateException {

        int i = 0;
        for (Map.Entry<String, Object> entry : entrySet) {
            NuclioIPC.Entry.Builder entryBuilder = builder.get(i);
            entryBuilder.setKey(entry.getKey());
            Object value = entry.getValue();
            NuclioIPC.Entry.Value.Builder valueBuilder = entryBuilder.initValue();
            if (value instanceof String) {
                valueBuilder.setSVal((String) value);
            } else if (value instanceof Integer) {
                valueBuilder.setIVal((Integer) value);
            } else if (value instanceof Long) {
                valueBuilder.setIVal((Long) value);
            } else if (value instanceof Float) {
                valueBuilder.setFVal((Float) value);
            } else if (value instanceof Double) {
                valueBuilder.setFVal((Double) value);
            } else if (value instanceof byte[]) {
                valueBuilder.setDVal((byte[]) value);
            } else {
                throw new IllegalArgumentException("unknown type: " + value.getClass());
            }
            i++;
        }
    }

    public static Set<Map.Entry<String, Object>> toEntrySet(Object... values) throws IllegalArgumentException {
        if (values.length % 2 != 0) {
            throw new IllegalArgumentException("values must have even length");
        }

        Set<Map.Entry<String, Object>> entrySet = new HashSet<Map.Entry<String, Object>>();
        for (int i = 0; i < values.length; i += 2) {
            String key;
            try {
                key = (String) values[i];
            } catch (ClassCastException e) {
                String message = String.format("values[%d]: %s is not a String", i, values[i]);
                throw new IllegalArgumentException(message);
            }

            Object value = values[i + 1];
            entrySet.add(new Entry<String, Object>(key, value));
        }

        return entrySet;
    }
}


class Entry<K, V> implements Map.Entry<K, V> {
    private K key;
    private V value;

    public Entry(K key, V value) {
        this.key = key;
        this.value = value;
    }

    @Override
    public K getKey() {
        return key;
    }

    @Override
    public V getValue() {
        return value;
    }

    @Override
    public V setValue(V value) {
        V old = this.value;
        this.value = value;
        return old;
    }
}

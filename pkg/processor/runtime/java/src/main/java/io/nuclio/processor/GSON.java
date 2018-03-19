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

import java.lang.reflect.Type;
import java.util.Base64;
import java.util.Date;

import com.google.gson.Gson;
import com.google.gson.GsonBuilder;
import com.google.gson.JsonDeserializationContext;
import com.google.gson.JsonDeserializer;
import com.google.gson.JsonElement;
import com.google.gson.JsonParseException;
import com.google.gson.JsonPrimitive;
import com.google.gson.JsonSerializationContext;
import com.google.gson.JsonSerializer;

public class GSON {
    public static Gson createGson() {
        return new GsonBuilder()
                .registerTypeHierarchyAdapter(byte[].class, new ByteArrayAdapter())
                .registerTypeHierarchyAdapter(Date.class, new DateAdapter())
                .create();

    }
}

class ByteArrayAdapter implements JsonSerializer<byte[]>, JsonDeserializer<byte[]> {
    @Override
    public byte[] deserialize(JsonElement json, Type typeOfT, JsonDeserializationContext context)
            throws JsonParseException {
        Base64.Decoder dec = Base64.getDecoder();
        return dec.decode(json.getAsString());
    }

    @Override
    public JsonElement serialize(byte[] src, Type typeOfSrc, JsonSerializationContext context) {
        Base64.Encoder enc = Base64.getEncoder();
        return new JsonPrimitive(enc.encodeToString(src));
    }
}

class DateAdapter implements JsonSerializer<Date>, JsonDeserializer<Date> {
    @Override
    public Date deserialize(JsonElement json, Type typeOfT, JsonDeserializationContext context)
            throws JsonParseException {
        return new Date(json.getAsLong() * 1000);
    }

    @Override
    public JsonElement serialize(Date src, Type typeOfSrc, JsonSerializationContext context) {
        return new JsonPrimitive(src.getTime() / 1000);
    }
}
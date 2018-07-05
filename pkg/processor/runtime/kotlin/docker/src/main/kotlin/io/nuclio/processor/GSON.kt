/*
Copyright 2018 The Nuclio Authors.

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

package io.nuclio.processor

import java.lang.reflect.Type
import java.util.Base64
import java.util.Date

import com.google.gson.Gson
import com.google.gson.GsonBuilder
import com.google.gson.JsonDeserializationContext
import com.google.gson.JsonDeserializer
import com.google.gson.JsonElement
import com.google.gson.JsonParseException
import com.google.gson.JsonPrimitive
import com.google.gson.JsonSerializationContext
import com.google.gson.JsonSerializer

object GSON {
    fun createGson(): Gson {
        return GsonBuilder()
                .registerTypeHierarchyAdapter(ByteArray::class.java, ByteArrayAdapter())
                .registerTypeHierarchyAdapter(Date::class.java, DateAdapter())
                .create()

    }
}

internal class ByteArrayAdapter : JsonSerializer<ByteArray>, JsonDeserializer<ByteArray> {
    @Throws(JsonParseException::class)
    override fun deserialize(json: JsonElement, typeOfT: Type, context: JsonDeserializationContext): ByteArray {
        val dec = Base64.getDecoder()
        return dec.decode(json.asString)
    }

    override fun serialize(src: ByteArray, typeOfSrc: Type, context: JsonSerializationContext): JsonElement {
        val enc = Base64.getEncoder()
        return JsonPrimitive(enc.encodeToString(src))
    }
}


internal class DateAdapter : JsonSerializer<Date>, JsonDeserializer<Date> {
    @Throws(JsonParseException::class)
    override fun deserialize(json: JsonElement, typeOfT: Type, context: JsonDeserializationContext): Date {
        return Date(json.asLong * 1000)
    }

    override fun serialize(src: Date, typeOfSrc: Type, context: JsonSerializationContext): JsonElement {
        return JsonPrimitive(src.time / 1000)
    }
}
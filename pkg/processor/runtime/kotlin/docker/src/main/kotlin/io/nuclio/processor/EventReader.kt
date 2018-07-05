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

import java.io.BufferedReader
import java.io.InputStream
import java.io.InputStreamReader

import com.google.gson.Gson

class EventReader(input: InputStream) {
    private var gson: Gson = GSON.createGson()
    private var reader: BufferedReader = BufferedReader(InputStreamReader(input))

    @Throws(Throwable::class)
    operator fun next(): JsonEvent? {
        val line = reader.readLine() ?: return null

        return gson.fromJson(line, JsonEvent::class.java)
    }
}

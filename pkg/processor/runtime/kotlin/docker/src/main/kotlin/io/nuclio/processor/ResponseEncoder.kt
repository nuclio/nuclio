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

import com.google.gson.Gson
import com.google.gson.GsonBuilder

import java.io.BufferedOutputStream
import java.io.OutputStream
import java.util.Date
import java.util.HashMap

class ResponseEncoder @Throws(Throwable::class)
constructor(out: OutputStream) {
    private val out: BufferedOutputStream = BufferedOutputStream(out)
    private var gson: Gson = GsonBuilder()
            .registerTypeHierarchyAdapter(ByteArray::class.java, ByteArrayAdapter())
            .registerTypeHierarchyAdapter(Date::class.java, DateAdapter())
            .create()


    @Throws(Throwable::class)
    fun encode(response: io.nuclio.Response) {
        out.write('r'.toInt())

        val jresp = HashMap<String, Any>()
        jresp["body"] = response.body
        jresp["status_code"] = response.statusCode
        jresp["content_type"] = response.contentType
        jresp["body_encoding"] = "base64"
        jresp["headers"] = response.headers

        this.out.write(gson.toJson(jresp).toByteArray())
        this.out.write('\n'.toInt())
        this.out.flush()
    }
}

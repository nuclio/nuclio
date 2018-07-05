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


// IF YOU MAKE CHANGES TO THIS FILE RUN "gradle jar" *BEFORE* RUNNING TESTS
// TODO: Find a way to automate this in the tests

import io.nuclio.Context
import io.nuclio.Event
import io.nuclio.EventHandler
import io.nuclio.Response
import java.util.*


class Outputter : EventHandler {
    override fun handleEvent(context: Context, event: Event): Response {
        if (event.method != "POST") {
            return Response().setBody(event.method)
        }

        val body = String(event.body)

        when (body) {
            "return_string" -> return Response().setBody("a string")
            "return_bytes" -> return Response().setBody("bytes".toByteArray())
            "log" -> {
                context.logger.debug("Debug message")
                context.logger.info("Info message")
                context.logger.warn("Warn message")
                context.logger.error("Error message")

                return Response().setBody("returned logs").setStatusCode(201)
            }
            "log_with" -> {
                context.logger.errorWith(
                        "Error message", "source", "rabbit", "weight", 7)
                return Response().setBody("returned logs with").setStatusCode(201)
            }
            "return_response" -> {
                val headers = HashMap<String, Any>()
                headers["h1"] = "v1"
                headers["h2"] = "v2"

                return Response().setBody("response body").setHeaders(headers)
                        .setContentType("text/plain").setStatusCode(201)
            }
            "return_fields" -> {
                val fields = ArrayList<String>()
                for ((key, value) in event.fields) {
                    fields.add(String.format("%s=%s", key, value))
                }
                fields.sort()

                val outBody = fields.joinToString(",")
                return Response().setBody(outBody)
            }
            "return_path" -> return Response().setBody(event.path)
            "return_error" -> throw RuntimeException("some error")
            else -> throw RuntimeException(String.format("Unknown return mode: %s", body))
        }

    }
}
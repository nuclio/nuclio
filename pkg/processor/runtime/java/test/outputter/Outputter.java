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


// IF YOU MAKE CHANGES TO THIS FILE RUN "gradle jar" *BEFORE* RUNNING TESTS
// TODO: Find a way to automate this in the tests

import io.nuclio.Context;
import io.nuclio.Event;
import io.nuclio.EventHandler;
import io.nuclio.Response;

import java.util.*;


public class Outputter implements EventHandler {
    @Override
    public Response handleEvent(Context context, Event event) {
        if (!event.getMethod().equals("POST")) {
            return new Response().setBody(event.getMethod());
        }

        String body = new String(event.getBody());

        if (body.equals("return_string")) {
            return new Response().setBody("a string");
        } else if (body.equals("return_bytes")) {
            return new Response().setBody("bytes".getBytes());
        } else if (body.equals("log")) {
            context.getLogger().debug("Debug message");
            context.getLogger().info("Info message");
            context.getLogger().warn("Warn message");
            context.getLogger().error("Error message");

            return new Response().setBody("returned logs").setStatusCode(201);
        } else if (body.equals("log_with")) {
            context.getLogger().errorWith(
                    "Error message", "source", "rabbit", "weight", 7);
            return new Response().setBody("returned logs with").setStatusCode(201);
        } else if (body.equals("return_response")) {
            Map<String, Object> headers = new HashMap<String, Object>();
            headers.put("h1", "v1");
            headers.put("h2", "v2");

            return new Response().setBody("response body").setHeaders(headers)
                    .setContentType("text/plain").setStatusCode(201);
        } else if (body.equals("return_fields")) {
            List<String> fields = new ArrayList<String>();
            for (Map.Entry<String, Object> entry : event.getFields().entrySet()) {
                fields.add(String.format("%s=%s", entry.getKey(), entry.getValue()));
            }
            Collections.sort(fields);

            String outBody = String.join(",", fields);
            return new Response().setBody(outBody);
        } else if (body.equals("return_path")) {
            return new Response().setBody(event.getPath());
        } else if (body.equals("return_error")) {
            throw new RuntimeException("some error");
        }

        throw new RuntimeException(String.format("Unknown return mode: %s", body));
    }
}

import io.nuclio.Context;
import io.nuclio.Event;
import io.nuclio.EventHandler;
import io.nuclio.Response;

import java.util.Arrays;
import java.util.Collections;
import java.util.List;
import java.util.Map;


public class Outputter implements EventHandler {
    @Override
    public Response handleEvent(Context context, Event event) {
        if (event.getMethod() != "POST") {
            return new Response().setBody(event.getMethod());
        }

        String body = new String(event.getBody());

        if (body == "return_string") {
            return new Response().setBody("a string");
        } else if (body == "return_bytes") {
            return new Response().setBody("bytes".getBytes());
        } else if (body == "log") {
            context.getLogger().debug("Debug message");
            context.getLogger().info("Info message");
            context.getLogger().warn("Warn message");
            context.getLogger().error("Error message");

            return new Response().setBody("returned logs").setStatusCode(201);
        } else if (body == "log_with") {
            context.getLogger().errorWith(
                    "Error message", "source", "rabbit", "weight", 7);
            return new Response().setBody("returned logs with").setStatusCode(201);
        } else if (body == "return response") {
            Map<String, Object> headers = Collections.emptyMap();
            headers.put("h1", "v1");
            headers.put("h1", "v2");

            return new Response().setBody("response body").setHeaders(headers)
                    .setContentType("text/plain").setStatusCode(201);
        } else if (body == "return fields") {
            List<String> fields = Collections.emptyList();
            for (Map.Entry<String, Object> entry : event.getFields().entrySet()) {
                fields.add(String.format("%s=%s", entry.getKey(), entry.getValue()));
            }
            Collections.sort(fields);

            String outBody = String.join(",", fields);
            return new Response().setBody(outBody);
        } else if (body == "return_path") {
            return new Response().setBody(event.getPath());
        } else if (body == "return_error") {
            throw new RuntimeException("some error");
        }

        throw new RuntimeException(String.format("Unknown return mode: %s", body));
    }
}

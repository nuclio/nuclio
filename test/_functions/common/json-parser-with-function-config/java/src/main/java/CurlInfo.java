import com.fasterxml.jackson.core.type.TypeReference;
import io.nuclio.Context;
import io.nuclio.Event;
import io.nuclio.EventHandler;
import io.nuclio.Response;

import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.util.HashMap;
import java.util.Map;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.annotation.JsonProperty;


public class CurlInfo implements EventHandler {
    @Override
    public Response handleEvent(Context context, Event event) {
        try {
            if (!curlInstalled()) {
                return new Response().setBody("curl not installed").setStatusCode(500);
            }

            ObjectMapper mapper = new ObjectMapper();
            TypeReference<HashMap<String, String>> typeRef = new TypeReference<HashMap<String, String>>() {
            };

            Map<String, String> request = mapper.readValue(event.getBody(), typeRef);
            return new Response().setBody(request.get("return_this"));
        } catch (Throwable err) {
            return new Response().setBody(err.toString()).setStatusCode(500);
        }
    }

    private boolean curlInstalled() throws Throwable {
        Process process = new ProcessBuilder("curl", "--version").start();
        return process.waitFor() == 0;
    }
}

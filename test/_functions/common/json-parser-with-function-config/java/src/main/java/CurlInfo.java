import io.nuclio.Context;
import io.nuclio.Event;
import io.nuclio.EventHandler;
import io.nuclio.Response;

import java.io.BufferedReader;
import java.io.InputStreamReader;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.annotation.JsonProperty;

class Request {
	@JsonProperty("return_this")
	String returnThis;
}


public class CurlInfo implements EventHandler {
	@Override
	public Response handleEvent(Context context, Event event) {
		try {
			if (!curlInstalled()) {
				return new Response().setBody("curl not installed").setStatusCode(500);
			}

			ObjectMapper mapper = new ObjectMapper();
			Request request = mapper.readValue(event.getBody(), Request.class);
			return new Response().setBody(request.returnThis);
		} catch (Throwable err) {
			return new Response().setBody(err.toString()).setStatusCode(500);
		}
	}

	private boolean curlInstalled() throws Throwable {
       Process process = new ProcessBuilder("curl", "--version").start();
		return process.exitValue() == 0;
	}
}

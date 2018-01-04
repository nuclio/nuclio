import io.nuclio.Context;
import io.nuclio.Event;
import io.nuclio.EventHandler;
import io.nuclio.Response;

import java.io.BufferedReader;
import java.io.InputStreamReader;


public class CurlInfo implements EventHandler {
    @Override
    public Response handleEvent(Context context, Event event) {
	try {
	   String version = curlVersion();
	   return new Response().setBody(version);
	} catch (Throwable e) {
	    return new Response().setBody(e.toString()).setStatusCode(500);
	}
    }

    private String curlVersion() throws Throwable {
	Process process = new ProcessBuilder("curl", "--version").start();

	BufferedReader reader = new BufferedReader(
		new InputStreamReader(process.getInputStream()));
	StringBuilder builder = new StringBuilder();

	String line;
	while ((line = reader.readLine()) != null) {
	    builder.append(line);
	}

	if (process.exitValue() != 0) {
	    throw new RuntimeException("Can't get curl version");
	}

	return builder.toString();
    }
}

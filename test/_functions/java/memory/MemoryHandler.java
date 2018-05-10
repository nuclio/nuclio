import io.nuclio.Context;
import io.nuclio.Event;
import io.nuclio.EventHandler;
import io.nuclio.Response;

public class MemoryHandler implements EventHandler {
    @Override
    public Response handleEvent(Context context, Event event) {
		String body = String.format("Max: %d", Runtime.getRuntime().maxMemory());
        return new Response().setBody(body);
    }
}


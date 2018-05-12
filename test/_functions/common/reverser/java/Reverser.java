import io.nuclio.Context;
import io.nuclio.Event;
import io.nuclio.EventHandler;
import io.nuclio.Response;

public class Reverser implements EventHandler {
    @Override
    public Response handleEvent(Context context, Event event) {
       String body = new String(event.getBody());
       String reversed = new StringBuilder(body).reverse().toString();
       return new Response().setBody(reversed);
    }
}

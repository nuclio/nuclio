import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.util.HashMap;
import java.util.Map;
import io.nuclio.Context;
import io.nuclio.Event;
import io.nuclio.EventHandler;
import io.nuclio.Response;


public class JsonParser implements EventHandler {

    @Override
    public Response handleEvent(Context context, Event event) {
        try {
            ObjectMapper mapper = new ObjectMapper();
            TypeReference< HashMap < String, String > > typeRef = new TypeReference < HashMap < String, String > >() {
            };

            Map< String, String > request = mapper.readValue(event.getBody(), typeRef);
            return new Response().setBody(request.get("return_this"));
        } catch (Throwable err) {
            return new Response().setBody(err.toString()).setStatusCode(500);
        }
    }
}

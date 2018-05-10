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

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

import com.fasterxml.jackson.core.JsonFactory;
import com.fasterxml.jackson.core.JsonGenerator;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.nuclio.Response;

import java.io.IOException;
import java.io.PrintWriter;
import java.util.Map;

public class ResponseEncoder {
    private PrintWriter out;
    private ObjectMapper mapper;
    JsonGenerator gen;


    public ResponseEncoder(PrintWriter out) throws Throwable{
        JsonFactory factory = new JsonFactory();
        this.out = out;
        this.gen = factory.createGenerator(out);
        this.mapper = new ObjectMapper();
        this.mapper.configure(JsonGenerator.Feature.AUTO_CLOSE_TARGET, false);
    }

    public void encode(Response response) throws IOException {

        this.out.write('r');

        this.gen.writeStartObject();
        this.gen.writeNumberField("status_code", response.getStatusCode());
        this.gen.writeStringField("content_type", response.getContentType());
        this.gen.writeBinaryField("body", response.getBody());
        this.gen.writeStringField("body_encoding", "base64");

        this.gen.writeFieldName("headers");
        this.gen.writeStartObject();
        for (Map.Entry<String, Object> entry : response.getHeaders().entrySet()) {
            this.gen.writeObjectField(entry.getKey(), entry.getValue());
        }
        this.gen.writeEndObject();

        this.gen.writeEndObject();
        this.gen.flush();

        this.out.println("");
    }
}

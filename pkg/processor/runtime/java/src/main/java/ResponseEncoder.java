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

import com.fasterxml.jackson.core.JsonGenerator;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.nuclio.Response;

import java.io.IOException;
import java.io.PrintWriter;
import java.util.HashMap;
import java.util.Map;
import java.util.Base64;

public class ResponseEncoder {
    private PrintWriter out;
    private ObjectMapper mapper;
    private Base64.Encoder base64Encoder = Base64.getEncoder();


    public ResponseEncoder(PrintWriter out) throws IOException {
        this.out = out;
        this.mapper = new ObjectMapper();
        this.mapper.configure(JsonGenerator.Feature.AUTO_CLOSE_TARGET, false);
    }

    public void encode(Response response) throws IOException {
        Map<String, Object> map = new HashMap<String, Object>();
        map.put("status_code", response.getStatusCode());
        map.put("content_type", response.getContentType());
        map.put("headers", response.getHeaders());

        String body = base64Encoder.encodeToString(response.getBody());
        map.put("body", body);
        map.put("body_encoding", "base64");

        this.out.write('r');
        this.mapper.writeValue(this.out, map);
        this.out.println("");
    }
}

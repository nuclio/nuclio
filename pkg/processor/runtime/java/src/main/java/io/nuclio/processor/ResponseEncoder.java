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

package io.nuclio.processor;

import com.google.gson.Gson;
import com.google.gson.GsonBuilder;

import java.io.BufferedOutputStream;
import java.io.OutputStream;
import java.util.Date;
import java.util.HashMap;
import java.util.Map;

public class ResponseEncoder {
    private BufferedOutputStream out;
    Gson gson;


    public ResponseEncoder(OutputStream out) throws Throwable {
        this.out = new BufferedOutputStream(out);

        this.gson = new GsonBuilder()
                .registerTypeHierarchyAdapter(byte[].class, new ByteArrayAdapter())
                .registerTypeHierarchyAdapter(Date.class, new DateAdapter())
                .create();
    }

    public void encode(io.nuclio.Response response) throws Throwable {
        this.out.write('r');

        Map<String, Object> jresp = new HashMap<String, Object>();
        jresp.put("body", response.getBody());
        jresp.put("status_code", response.getStatusCode());
        jresp.put("content_type", response.getContentType());
        jresp.put("body_encoding", "base64");
        jresp.put("headers", response.getHeaders());

        this.out.write(gson.toJson(jresp).getBytes());
        this.out.write('\n');
        this.out.flush();
    }
}

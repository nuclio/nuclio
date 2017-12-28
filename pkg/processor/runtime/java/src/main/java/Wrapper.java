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

import io.nuclio.wrapper.NuclioIPC;
import io.nuclio.Response;
import io.nuclio.Event;

import java.io.*;
import java.nio.*;
import java.nio.channels.*;
import java.nio.charset.*;
import java.nio.file.*;
import java.util.*;

import org.capnproto.*;

public class Wrapper {
    public static MessageBuilder encodeResp(Response resp) throws Exception {
        MessageBuilder message = new org.capnproto.MessageBuilder();

        NuclioIPC.Response.Builder rb =
                message.initRoot(NuclioIPC.Response.factory);
        rb.setBody(resp.getBody());
        rb.setStatus(resp.getStatusCode());
        rb.setContentType(resp.getContentType());

        Map<String, Object> headers = resp.getHeaders();
        StructList.Builder<NuclioIPC.Entry.Builder> hb =
                rb.initHeaders(headers.size());


        int i = 0;
        for (Map.Entry<String, Object> entry : headers.entrySet()) {
            NuclioIPC.Entry.Builder eb = hb.get(i);
            eb.setKey(entry.getKey());
            Object value = entry.getValue();
            NuclioIPC.Entry.Value.Builder vb = eb.initValue();
            if (value instanceof String) {
                vb.setSVal((String) value);
            } else if (value instanceof Integer) {
                vb.setIVal((Integer) value);
            } else if (value instanceof byte[]) {
                vb.setDVal((byte[]) value);
            } else {
                throw new RuntimeException("unknown type: " + value.getClass());
            }
            i++;
        }
        return message;
    }

    private static void writeResponse(Response resp, FileChannel chan) throws Throwable {
        MessageBuilder msg = encodeResp(resp);

        chan.position(0);
        Serialize.write(chan, msg);
        chan.force(true);
    }

    private static void readEvent(MappedByteBuffer buf) throws Exception {
        buf.position(0);
        MessageReader reader = Serialize.read(buf);

        NuclioIPC.Event.Reader eventReader = reader.getRoot(NuclioIPC.Event.factory);
        Event event = CapnpEvent.fromReader(eventReader);
        //System.out.println(event.getPath());
    }

    public static void main(String[] args) throws Throwable {
        if (args.length > 0) {
            switch (args[0]) {
                case "-h":
                case "--help":
                    System.out.println("usage: shmem IN-PIPE OUT-PIPE DATA-FILE");
                    System.exit(0);
            }
        }
        if (args.length != 3) {
            System.err.println("error: wrong number of arguments");
            System.exit(1);
        }

        PipeReader in = new PipeReader(args[0]);
        FileWriter out = new FileWriter(args[1]);
        //System.out.println("data: " + args[2]);
        File file = new File(args[2]);
        FileChannel chan = FileChannel.open(
                file.toPath(), StandardOpenOption.READ, StandardOpenOption.WRITE);
        MappedByteBuffer buf = chan.map(FileChannel.MapMode.READ_WRITE, 0, file.length());

        Map<String, Object> headers = new HashMap<String, Object>();
        headers.put("h10", "v10");
        headers.put("h11", 11);
        Response resp = new Response().setBody("OK".getBytes()).setStatusCode(200)
                .setHeaders(headers);

        while (true) {
            in.read();
            //System.out.println("Got event");
            readEvent(buf);
            writeResponse(resp, chan);
            out.write('r');
            out.flush();
        }
    }

}

class PipeReader {
    private String path;
    private FileInputStream in;

    public PipeReader(String path) {
        this.path = path;
    }

    public int read() throws Throwable {
        // We create file here since it'll block initially on named pipe
        if (this.in == null) {
            this.in = new FileInputStream(this.path);
        }

        while (true) {
            int ch = this.in.read();
            if (ch != -1) {
                return ch;
            }
            Thread.sleep(0);
        }
    }
}

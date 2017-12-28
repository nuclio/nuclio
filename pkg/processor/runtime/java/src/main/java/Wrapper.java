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

import io.nuclio.Context;
import io.nuclio.Event;
import io.nuclio.EventHandler;
import io.nuclio.Logger;
import io.nuclio.Response;


import java.io.File;
import java.io.FileInputStream;
import java.io.FileWriter;
import java.lang.reflect.Constructor;
import java.lang.reflect.Method;
import java.net.URL;
import java.net.URLClassLoader;
import java.nio.MappedByteBuffer;
import java.nio.channels.FileChannel;
import java.nio.file.StandardOpenOption;
import java.util.HashMap;
import java.util.Map;

import org.capnproto.MessageBuilder;
import org.capnproto.MessageReader;
import org.capnproto.Serialize;
import org.capnproto.StructList;

import org.apache.commons.cli.*;

public class Wrapper {
    private static MessageBuilder encodeResp(Response resp) throws Exception {
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

    private static Event readEvent(MappedByteBuffer buf) throws Exception {
        buf.position(0);
        MessageReader reader = Serialize.read(buf);

        NuclioIPC.Event.Reader eventReader = reader.getRoot(NuclioIPC.Event.factory);
        return CapnpEvent.fromReader(eventReader);
    }

    private static EventHandler loadHandler(String jarPath, String handlerClassName) throws Throwable {
        URL[] loaderUrls = new URL[]{
                new URL("file://" + jarPath),
        };
        URLClassLoader loader = new URLClassLoader(loaderUrls);
        Class<?> cls = loader.loadClass(handlerClassName);
        Constructor<?> constructor = cls.getConstructor();
        Object obj = constructor.newInstance();
        return (EventHandler) obj;
    }

    private static Options buildOptions() {
        String[][] optsArray = {
                {"data", "data file path"},
                {"handler", "handler class name"},
                {"in", "input pipe path"},
                {"jar", "jar file path"},
                {"out", "output pipe path"},
        };

        Options options = new Options();
        for (String[] opt : optsArray) {
            options.addOption(
                    Option.builder(opt[0]).required().hasArg().desc(opt[1]).build());
        }

        return options;
    }


    public static void main(String[] args) throws Throwable {
        Options options = buildOptions();

        CommandLineParser parser = new DefaultParser();
        HelpFormatter formatter = new HelpFormatter();
        CommandLine cmd;

        try {
            cmd = parser.parse(options, args);
        } catch (ParseException e) {
            System.out.println(e.getMessage());
            new HelpFormatter().printHelp("t", options);
            System.exit(1);
            return;
        }

        String dataPath = cmd.getOptionValue("data");
        String handlerClassName = cmd.getOptionValue("handler");
        String inPath = cmd.getOptionValue("in");
        String jarPath = cmd.getOptionValue("jar");
        String outPath = cmd.getOptionValue("out");

        PipeReader in = new PipeReader(inPath);
        FileWriter out = new FileWriter(outPath);
        File file = new File(dataPath);
        FileChannel chan = FileChannel.open(
                file.toPath(), StandardOpenOption.READ, StandardOpenOption.WRITE);
        MappedByteBuffer buf = chan.map(FileChannel.MapMode.READ_WRITE, 0, file.length());

        EventHandler handler = loadHandler(jarPath, handlerClassName);
        Context context = new WrapperContext();

        while (true) {
            in.read();
            //System.out.println("Got event");
            Event event = readEvent(buf);
            // TODO: try/catch
            Response response = handler.handleEvent(context, event);
            writeResponse(response, chan);
            out.write('r');
            out.flush();
        }
    }
}

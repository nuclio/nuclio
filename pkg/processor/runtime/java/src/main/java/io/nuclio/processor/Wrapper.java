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

import io.nuclio.Context;
import io.nuclio.Event;
import io.nuclio.EventHandler;
import io.nuclio.Response;


import java.io.*;
import java.lang.reflect.Constructor;
import java.net.Socket;
import java.text.SimpleDateFormat;
import java.util.Date;

import org.apache.commons.cli.*;

public class Wrapper {
    private static boolean verbose = false;
    private static SimpleDateFormat dateFormat;
    private static String usage = "wrapper -handler HANDLER -port PORT";

    static {
        dateFormat = new SimpleDateFormat("yyyy-MM-dd HH:mm:ss");
    }

    /**
     * Print debugging log message to stdout
     *
     * @param format Message format
     * @param args   Message arguments
     */
    private static void debugLog(String format, Object... args) {
        if (!verbose) {
            return;
        }

        Date now = new Date();
        String message = String.format(format, args);


        System.out.println(String.format("[%s] %s", dateFormat.format(now), message));
    }

    /**
     * Load Event handler
     * <p>
     * We assume the handler code is in the same jar as this
     *
     * @param handlerClassName Handler class name
     * @return Handler
     * @throws Throwable
     */
    private static EventHandler loadHandler(String handlerClassName) throws Throwable {
        ClassLoader loader = Wrapper.class.getClassLoader();
        Class<?> cls = loader.loadClass(handlerClassName);
        Constructor<?> constructor = cls.getConstructor();
        Object obj = constructor.newInstance();
        return (EventHandler) obj;
    }

    /**
     * Build command line options
     *
     * @return Options
     */
    private static Options buildOptions() {
        String[][] optsArray = {
                {"handler", "handler class name"},
                {"port", "communication port"},
        };

        Options options = new Options();
        for (String[] opt : optsArray) {
            options.addOption(
                    Option.builder(opt[0]).required().hasArg().desc(opt[1]).build());
        }
        options.addOption(
                Option.builder("verbose").desc("emit debug information").build());

        return options;
    }

    /**
     * Parse port value from String to int
     *
     * @param portValue port String value
     * @return port as int, -1 on failure
     */
    private static int parsePort(String portValue) {
        int port = 0;
        try {
            return Integer.parseInt(portValue);
        } catch (ClassCastException e) {
            return -1;
        }
    }

    public static void main(String[] args) throws Throwable {
        Options options = buildOptions();

        CommandLineParser parser = new DefaultParser();
        CommandLine cmd;

        try {
            cmd = parser.parse(options, args);
        } catch (ParseException e) {
            System.out.println(e.getMessage());
            new HelpFormatter().printHelp(usage, options);
            System.exit(1);
            return;
        }

        verbose = cmd.hasOption("verbose");

        String handlerClassName = cmd.getOptionValue("handler");
        debugLog("handler: %s", handlerClassName);

        String portValue = cmd.getOptionValue("port");
        int port = parsePort(portValue);
        if (port <= 0) {
            System.err.format("error: bad port - %s", portValue);
            System.exit(1);
        }

        debugLog("port: %d", port);

        EventHandler handler = loadHandler(handlerClassName);
        debugLog("Handler %s loaded", handlerClassName);

        Socket sock = new Socket("localhost", port);
        ResponseEncoder responseEncoder = new ResponseEncoder(sock.getOutputStream());
        EventReader eventReader = new EventReader(sock.getInputStream());

        Context context = new WrapperContext(sock.getOutputStream());
        Response response;

        while (true) {
            try {
                Event event = eventReader.next();
                if (event == null) {
                    break;
                }
                response = handler.handleEvent(context, event);
            } catch (Exception err) {
                StringWriter stringWriter = new StringWriter();
                PrintWriter printWriter = new PrintWriter(stringWriter);
                printWriter.format("Error in handler: %s\n", err.toString());
                err.printStackTrace(printWriter);
                printWriter.flush();

                response = new Response().setBody(stringWriter.toString())
                        .setStatusCode(500);
            }
            responseEncoder.encode(response);
        }
    }

}

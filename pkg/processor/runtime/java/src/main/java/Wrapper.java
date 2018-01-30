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

import io.nuclio.Context;
import io.nuclio.Event;
import io.nuclio.EventHandler;
import io.nuclio.Response;


import java.io.*;
import java.lang.reflect.Constructor;
import java.net.Socket;
import java.net.URL;
import java.net.URLClassLoader;
import java.text.SimpleDateFormat;
import java.util.Date;

import org.apache.commons.cli.*;

public class Wrapper {
    private static boolean verbose = false;
    private static SimpleDateFormat dateFormat;

    static {
        dateFormat = new SimpleDateFormat("yyyy-MM-dd HH:mm:ss");
    }

    /**
     * Print debugging log message to stdout
     *
     * @param format Message format
     * @param args Message arguments
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
     *
     * @param jarPath Path to handler JAR
     * @param handlerClassName Handler class name
     * @return Handler
     * @throws Throwable
     */
    private static EventHandler loadHandler(String jarPath, String handlerClassName) throws Throwable {
        URL[] loaderUrls = new URL[]{
                new URL("file://" + jarPath),
        };
        URLClassLoader loader = new URLClassLoader(loaderUrls);
        try {
            Class<?> cls = loader.loadClass(handlerClassName);
            Constructor<?> constructor = cls.getConstructor();
            Object obj = constructor.newInstance();
            return (EventHandler) obj;
        } finally {
            loader.close();
        }
    }

    /**
     * Build command line options
     * @return Options
     */
    private static Options buildOptions() {
        String[][] optsArray = {
                {"handler", "handler class name"},
                {"jar", "jar file path"},
                {"port", "communication port"},
        };

        Options options = new Options();
        for (String[] opt : optsArray) {
            options.addOption(
                    Option.builder(opt[0]).required().hasArg().desc(opt[1]).build());
        }
        options.addOption(Option.builder("verbose").build());

        return options;
    }

    public static void main(String[] args) throws Throwable {
        Options options = buildOptions();

        CommandLineParser parser = new DefaultParser();
        CommandLine cmd;

        try {
            cmd = parser.parse(options, args);
        } catch (ParseException e) {
            System.out.println(e.getMessage());
            new HelpFormatter().printHelp(args[0], options);
            System.exit(1);
            return;
        }

        verbose = cmd.hasOption("verbose");

        String jarPath = cmd.getOptionValue("jar");
        debugLog("jarPath: %s", jarPath);
        String handlerClassName = cmd.getOptionValue("handler");
        debugLog("handler: %s", handlerClassName);

        String portValue = cmd.getOptionValue("port");
        int port = 0;
        boolean portOK = false;

        try {
            port = Integer.parseInt(portValue);
            portOK = true;
        } catch (ClassCastException e) {

        }

        if ((!portOK) || (port <= 0)) {
            String message = String.format("error: bad port - %s", portValue);
            System.err.println(message);
            System.exit(1);
        }

        EventHandler handler = loadHandler(jarPath, handlerClassName);
        debugLog("Handler %s loaded from %s", handlerClassName, jarPath);

        Socket sock = new Socket("localhost", port);
        ResponseEncoder responseEncoder = new ResponseEncoder(sock.getOutputStream());
        EventReader eventReader = new EventReader(sock.getInputStream());

        Context context = new WrapperContext(sock.getOutputStream());
        Response response;

        while (true){
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

                response = new Response().setBody(stringWriter.toString())
                        .setStatusCode(500);
            }
            responseEncoder.encode(response);
        }
    }

}

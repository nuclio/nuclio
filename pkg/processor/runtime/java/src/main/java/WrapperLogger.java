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
import io.nuclio.Logger;

import java.io.IOException;
import java.io.PrintWriter;
import java.util.Collections;
import java.util.Map;
import java.util.Date;

public class WrapperLogger implements Logger {
    private PrintWriter out;
    private ObjectMapper mapper;

    /**
     * Encode with array to map
     *
     * @param with with object array
     * @return Map of key->value
     */
    private Map<String, Object> encodeWith(Object... with) {

        Map<String, Object> withMap = Collections.emptyMap();
        if (with.length % 2 != 0) {
            System.err.println(
                String.format("error: bad width length - %d", with.length));
            return withMap;
        }

        for (int i = 0; i < with.length; i+=2) {
            try {
                String key = (String) with[i];
                withMap.put(key, with[i + 1]);
            } catch (ClassCastException e) {
                String errorMessage = String.format(
                        "error: with[%d] is not a string - %s", i, with[i]);
                System.err.println(errorMessage);
            }
        }

        return withMap;
    }

    /**
     * Encode log in JSON format to out
     *
     * @param level Log level
     * @param message Log message
     * @param with With parameters
     */
    private void log(LogLevel level, String message, Object... with) {
        Map<String, Object> map = Collections.emptyMap();
        map.put("level", level.toString());
        map.put("message", message);
        map.put("datetime", new Date().toString());
        map.put("with", encodeWith(with));

        try {
            this.out.write('l');
            this.mapper.writeValue(this.out, map);
            this.out.println("");
        } catch (IOException e) {
            String error = String.format("error: can't encode log - %s", e);
            System.err.println(error);
            e.printStackTrace(System.err);
        }
    }


    public WrapperLogger(PrintWriter out) {
        this.out = out;

        this.mapper = new ObjectMapper();
        this.mapper.configure(JsonGenerator.Feature.AUTO_CLOSE_TARGET, false);
    }

    /**
     * Log an error message
     * e.g. logger.Error("%s not responding after %d seconds", dbHost, timeout)
     *
     * @param format Message format
     * @param args   formatting arguments
     */
    @Override
    public void error(String format, Object... args) {
        String message = String.format(format, args);
        log(LogLevel.ERROR, message);
    }

    /**
     * Log a warning message
     * e.g. logger.Warn("%s %.2f full", "memory", mem_full)
     *
     * @param format Message format
     * @param args   formatting arguments
     */
    @Override
    public void warn(String format, Object... args) {
        String message = String.format(format, args);
        log(LogLevel.WARNING, message);
    }

    /**
     * Log an info message
     * e.g. logger.Info("event with %d bytes", event.GetSize())
     *
     * @param format Message format
     * @param args   formatting arguments
     */
    @Override
    public void info(String format, Object... args) {
        String message = String.format(format, args);
        log(LogLevel.INFO, message);
    }

    /**
     * Log a debug message
     * e.g. logger.Debug("event with %d bytes", event.GetSize())
     *
     * @param format Message format
     * @param args   formatting arguments
     */
    @Override
    public void debug(String format, Object... args) {
        String message = String.format(format, args);
        log(LogLevel.DEBUG, message);
    }

    /**
     * Log a structured error message
     * e.g. logger.ErrorWith("bad request", "error", "daffy not found", "time", 7)
     *
     * @param format Message format
     * @param with   formatting arguments
     */
    @Override
    public void errorWith(String format, Object... with) {
        log(LogLevel.ERROR, format, with);
    }

    /**
     * Log a structured warning message
     * e.g. logger.WarnWith("system overload", "resource", "memory", "used", 0.9)
     *
     * @param format Message format
     * @param with   formatting arguments
     */
    @Override
    public void warnWith(String format, Object... with) {
        log(LogLevel.WARNING, format, with);
    }

    /**
     * Log a structured info message
     * e.g. logger.InfoWith("event processed", "time", 0.3, "count", 9009)
     *
     * @param format Message format
     * @param with   formatting arguments
     */
    @Override
    public void infoWith(String format, Object... with) {
        log(LogLevel.INFO, format, with);
    }

    /**
     * Log a structured debug message
     * e.g. logger.DebugWith("event", "body_size", 2339, "content-type", "text/plain")
     *
     * @param format Message format
     * @param with   formatting arguments
     */
    @Override
    public void debugWith(String format, Object... with) {
        log(LogLevel.DEBUG, format, with);
    }
}

enum LogLevel {
    ERROR("error"),
    WARNING("warning"),
    INFO("info"),
    DEBUG("debug"),
    ;

    private String text;

    /**
     * Set text value
     *
     * @param text
     */
    private LogLevel(String text) {
        this.text = text;
    }

    @Override
    public String toString() {
        return text;
    }
}

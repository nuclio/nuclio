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

import io.nuclio.Logger;
import io.nuclio.wrapper.NuclioIPC.LogRecord;
import io.nuclio.wrapper.NuclioIPC.Entry;
import org.capnproto.MessageBuilder;
import org.capnproto.Serialize;
import org.capnproto.StructList;

import java.io.FileWriter;
import java.nio.channels.FileChannel;

public class WrapperLogger implements Logger {
    private FileChannel chan;
    private FileWriter out;

    /**
     * Encode log in capnp format to chan and signal to out
     *
     * @param level Log level
     * @param logMessage Log message
     * @param with With parameters
     */
    private void log(LogRecord.Level level, String logMessage, Object ...with) {
        MessageBuilder message = new org.capnproto.MessageBuilder();

        LogRecord.Builder logBuilder = message.initRoot(LogRecord.factory);
        logBuilder.setLevel(level);
        logBuilder.setMessage(logMessage);

        StructList.Builder<Entry.Builder> withBuilder =
                logBuilder.initWith(with.length / 2);

        try {
            CapnpUtils.encodeEntrySet(withBuilder, CapnpUtils.toEntrySet(with));

            chan.position(0);
            Serialize.write(chan, message);
            chan.force(true);

            out.write('l');
            out.flush();
        } catch (Exception err) {
            System.err.println("ERROR: Can't encode " + err.toString());
            err.printStackTrace(System.err);
        }
    }

    public WrapperLogger(FileChannel chan, FileWriter out) {
        this.chan = chan;
        this.out = out;
    }

    /**
     * Log an error message
     * e.g. ctx.Error("%s not responding after %d seconds", dbHost, timeout)
     *
     * @param format Message format
     * @param args   formatting arguments
     */
    @Override
    public void error(String format, Object... args) {
        String logMessage = String.format(format, args);
        log(LogRecord.Level.ERROR, logMessage);
    }

    /**
     * Log a warning message
     * e.g. ctx.Warn("%s %.2f full", "memory", mem_full)
     *
     * @param format Message format
     * @param args   formatting arguments
     */
    @Override
    public void warn(String format, Object... args) {
        String logMessage = String.format(format, args);
        log(LogRecord.Level.WARNING, logMessage);
    }

    /**
     * Log an info message
     * e.g. ctx.Info("event with %d bytes", event.GetSize())
     *
     * @param format Message format
     * @param args   formatting arguments
     */
    @Override
    public void info(String format, Object... args) {
        String logMessage = String.format(format, args);
        log(LogRecord.Level.INFO, logMessage);
    }

    /**
     * Log a debug message
     * e.g. ctx.Debug("event with %d bytes", event.GetSize())
     *
     * @param format Message format
     * @param args   formatting arguments
     */
    @Override
    public void debug(String format, Object... args) {
        String logMessage = String.format(format, args);
        log(LogRecord.Level.DEBUG, logMessage);
    }

    /**
     * Log a structured error message
     * e.g. ctx.ErrorWith("bad request", "error", "daffy not found", "time", 7)
     *
     * @param format Message format
     * @param with   formatting arguments
     */
    @Override
    public void errorWith(String format, Object... with) {
        log(LogRecord.Level.ERROR, format, with);
    }

    /**
     * Log a structured warning message
     * e.g. ctx.WarnWith("system overload", "resource", "memory", "used", 0.9)
     *
     * @param format Message format
     * @param with   formatting arguments
     */
    @Override
    public void warnWith(String format, Object... with) {
        log(LogRecord.Level.WARNING, format, with);
    }

    /**
     * Log a structured info message
     * e.g. ctx.InfoWith("event processed", "time", 0.3, "count", 9009)
     *
     * @param format Message format
     * @param with   formatting arguments
     */
    @Override
    public void infoWith(String format, Object... with) {
        log(LogRecord.Level.INFO, format, with);
    }

    /**
     * Log a structured debug message
     * e.g. ctx.DebugWith("event", "body_size", 2339, "content-type", "text/plain")
     *
     * @param format Message format
     * @param with   formatting arguments
     */
    @Override
    public void debugWith(String format, Object... with) {
        log(LogRecord.Level.DEBUG, format, with);
    }
}

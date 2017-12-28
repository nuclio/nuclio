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


// TODO:
public class WrapperLogger implements Logger {
    /**
     * Log an error message
     * e.g. ctx.Error("%s not responding after %d seconds", dbHost, timeout)
     *
     * @param format Message format
     * @param args   formatting arguments
     */
    public void error(String format, Object... args) {
        System.out.println("ERROR: " + format);
    }

    /**
     * Log a warning message
     * e.g. ctx.Warn("%s %.2f full", "memory", mem_full)
     *
     * @param format Message format
     * @param args   formatting arguments
     */
    public void warn(String format, Object... args) {
        System.out.println("WARN: " + format);
    }

    /**
     * Log an info message
     * e.g. ctx.Info("event with %d bytes", event.GetSize())
     *
     * @param format Message format
     * @param args   formatting arguments
     */
    public void info(String format, Object... args) {
        System.out.println("INFO: " + format);
    }

    /**
     * Log a debug message
     * e.g. ctx.Debug("event with %d bytes", event.GetSize())
     *
     * @param format Message format
     * @param args   formatting arguments
     */
    public void debug(String format, Object... args) {
        System.out.println("DEBUG: " + format);
    }

    /**
     * Log a structured error message
     * e.g. ctx.ErrorWith("bad request", "error", "daffy not found", "time", 7)
     *
     * @param format Message format
     * @param args   formatting arguments
     */
    public void errorWith(String format, Object... vars) {
        System.out.println("ERROR WITH: " + format);
    }

    /**
     * Log a structured warning message
     * e.g. ctx.WarnWith("system overload", "resource", "memory", "used", 0.9)
     *
     * @param format Message format
     * @param args   formatting arguments
     */
    public void warnWith(String format, Object... vars) {
        System.out.println("WARN WITH: " + format);
    }

    /**
     * Log a structured info message
     * e.g. ctx.InfoWith("event processed", "time", 0.3, "count", 9009)
     *
     * @param format Message format
     * @param args   formatting arguments
     */
    public void infoWith(String format, Object... vars) {
        System.out.println("INFO WITH: " + format);
    }

    /**
     * Log a structured debug message
     * e.g. ctx.DebugWith("event", "body_size", 2339, "content-type", "text/plain")
     *
     * @param format Message format
     * @param args   formatting arguments
     */
    public void debugWith(String format, Object... vars) {
        System.out.println("DEBUG WITH: " + format);
    }
}

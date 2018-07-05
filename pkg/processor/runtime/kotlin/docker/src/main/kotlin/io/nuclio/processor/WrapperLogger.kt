/*
Copyright 2018 The Nuclio Authors.

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

package io.nuclio.processor

import com.google.gson.Gson
import io.nuclio.Logger

import java.io.BufferedOutputStream
import java.io.IOException
import java.io.OutputStream
import java.util.Date
import java.util.HashMap

class WrapperLogger(out: OutputStream) : Logger {
    private val gson: Gson = Gson()
    private val out: BufferedOutputStream = BufferedOutputStream(out)

    /**
     * Encode with array to map
     *
     * @param with with object array
     * @return Map of key->value
     */
    private fun encodeWith(vararg with: Any): Map<String, Any> {

        val withMap = HashMap<String, Any>()
        if (with.size % 2 != 0) {
            System.err.println(
                    String.format("error: bad width length - %d", with.size))
            return withMap
        }

        var i = 0
        while (i < with.size) {
            try {
                val key = with[i] as String
                withMap[key] = with[i + 1]
            } catch (e: ClassCastException) {
                val errorMessage = String.format(
                        "error: with[%d] is not a string - %s", i, with[i])
                System.err.println(errorMessage)
            }

            i += 2
        }

        return withMap
    }

    /**
     * Encode log in JSON format to out
     *
     * @param level   Log level
     * @param message Log message
     * @param with    With parameters
     */
    private fun log(level: LogLevel, message: String, vararg with: Any) {
        val log = Log(level, message, encodeWith(*with))

        try {
            this.out.write('l'.toInt())
            this.out.write(gson.toJson(log).toByteArray())
            this.out.write('\n'.toInt())
            this.out.flush()
        } catch (e: IOException) {
            val error = String.format("error: can't encode log - %s", e)
            System.err.println(error)
            e.printStackTrace(System.err)
        }

    }


    /**
     * Log an error message
     * e.g. logger.Error("%s not responding after %d seconds", dbHost, timeout)
     *
     * @param format Message format
     * @param args   formatting arguments
     */
    override fun error(format: String, vararg args: Any) {
        val message = String.format(format, *args)
        log(LogLevel.ERROR, message)
    }

    /**
     * Log a warning message
     * e.g. logger.Warn("%s %.2f full", "memory", mem_full)
     *
     * @param format Message format
     * @param args   formatting arguments
     */
    override fun warn(format: String, vararg args: Any) {
        val message = String.format(format, *args)
        log(LogLevel.WARNING, message)
    }

    /**
     * Log an info message
     * e.g. logger.Info("event with %d bytes", event.GetSize())
     *
     * @param format Message format
     * @param args   formatting arguments
     */
    override fun info(format: String, vararg args: Any) {
        val message = String.format(format, *args)
        log(LogLevel.INFO, message)
    }

    /**
     * Log a debug message
     * e.g. logger.Debug("event with %d bytes", event.GetSize())
     *
     * @param format Message format
     * @param args   formatting arguments
     */
    override fun debug(format: String, vararg args: Any) {
        val message = String.format(format, *args)
        log(LogLevel.DEBUG, message)
    }

    /**
     * Log a structured error message
     * e.g. logger.ErrorWith("bad request", "error", "daffy not found", "time", 7)
     *
     * @param format Message format
     * @param with   formatting arguments
     */
    override fun errorWith(format: String, vararg with: Any) {
        log(LogLevel.ERROR, format, *with)
    }

    /**
     * Log a structured warning message
     * e.g. logger.WarnWith("system overload", "resource", "memory", "used", 0.9)
     *
     * @param format Message format
     * @param with   formatting arguments
     */
    override fun warnWith(format: String, vararg with: Any) {
        log(LogLevel.WARNING, format, *with)
    }

    /**
     * Log a structured info message
     * e.g. logger.InfoWith("event processed", "time", 0.3, "count", 9009)
     *
     * @param format Message format
     * @param with   formatting arguments
     */
    override fun infoWith(format: String, vararg with: Any) {
        log(LogLevel.INFO, format, *with)
    }

    /**
     * Log a structured debug message
     * e.g. logger.DebugWith("event", "body_size", 2339, "content-type", "text/plain")
     *
     * @param format Message format
     * @param with   formatting arguments
     */
    override fun debugWith(format: String, vararg with: Any) {
        log(LogLevel.DEBUG, format, *with)
    }
}

internal enum class LogLevel
/**
 * Set text value
 *
 * @param text
 */
(private val text: String) {
    ERROR("error"),
    WARNING("warning"),
    INFO("info"),
    DEBUG("debug");

    override fun toString(): String {
        return text
    }
}

internal class Log(level: LogLevel, var message: String, var with: Map<String, Any>) {
    var level: String = level.toString()
    var datetime: String = Date().toString()
}
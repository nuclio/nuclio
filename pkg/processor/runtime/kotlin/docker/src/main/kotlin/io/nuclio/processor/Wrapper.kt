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


import io.nuclio.EventHandler
import io.nuclio.Response
import org.apache.commons.cli.*
import java.io.PrintWriter
import java.io.StringWriter
import java.net.Socket
import java.text.SimpleDateFormat
import java.util.*

object Wrapper {
    private var verbose = false
    private var dateFormat: SimpleDateFormat? = null
    private const val usage = "wrapper -handler HANDLER -port PORT"

    init {
        dateFormat = SimpleDateFormat("yyyy-MM-dd HH:mm:ss")
    }

    /**
     * Print debugging log message to stdout
     *
     * @param format Message format
     * @param args   Message arguments
     */
    private fun debugLog(format: String, vararg args: Any) {
        if (!verbose) {
            return
        }

        val now = Date()
        val message = String.format(format, *args)


        println(String.format("[%s] %s", dateFormat!!.format(now), message))
    }

    /**
     * Load Event handler
     *
     *
     * We assume the handler code is in the same jar as this
     *
     * @param handlerClassName Handler class name
     * @return Handler
     * @throws Throwable
     */
    @Throws(Throwable::class)
    private fun loadHandler(handlerClassName: String): EventHandler {
        val loader = Wrapper::class.java.classLoader
        val cls = loader.loadClass(handlerClassName)
        val constructor = cls.getConstructor()
        val obj = constructor.newInstance()
        return obj as EventHandler
    }

    /**
     * Build command line options
     *
     * @return Options
     */
    private fun buildOptions(): Options {
        val optsArray = arrayOf(arrayOf("handler", "handler class name"), arrayOf("port", "communication port"))

        val options = Options()
        for (opt in optsArray) {
            options.addOption(
                    Option.builder(opt[0]).required().hasArg().desc(opt[1]).build())
        }
        options.addOption(
                Option.builder("verbose").desc("emit debug information").build())

        return options
    }

    /**
     * Parse port value from String to int
     *
     * @param portValue port String value
     * @return port as int, -1 on failure
     */
    private fun parsePort(portValue: String): Int {
        return try {
            Integer.parseInt(portValue)
        } catch (e: ClassCastException) {
            -1
        }

    }

    @Throws(Throwable::class)
    @JvmStatic
    fun main(args: Array<String>) {
        val options = buildOptions()

        val parser = DefaultParser()
        val cmd: CommandLine

        try {
            cmd = parser.parse(options, args)
        } catch (e: ParseException) {
            println(e.message)
            HelpFormatter().printHelp(usage, options)
            System.exit(1)
            return
        }

        verbose = cmd.hasOption("verbose")

        val handlerClassName = cmd.getOptionValue("handler")
        debugLog("handler: %s", handlerClassName)

        val portValue = cmd.getOptionValue("port")
        val port = parsePort(portValue)
        if (port <= 0) {
            System.err.format("error: bad port - %s", portValue)
            System.exit(1)
        }

        debugLog("port: %d", port)

        val handler = loadHandler(handlerClassName)
        debugLog("Handler %s loaded", handlerClassName)

        val sock = Socket("localhost", port)
        val responseEncoder = ResponseEncoder(sock.getOutputStream())
        val eventReader = EventReader(sock.getInputStream())

        val context = WrapperContext(sock.getOutputStream())
        var response: Response

        while (true) {
            response = try {
                val event = eventReader.next() ?: break
                handler.handleEvent(context, event)
            } catch (err: Exception) {
                val stringWriter = StringWriter()
                val printWriter = PrintWriter(stringWriter)
                printWriter.format("Error in handler: %s\n", err.toString())
                err.printStackTrace(printWriter)
                printWriter.flush()

                Response().setBody(stringWriter.toString())
                        .setStatusCode(500)
            }

            responseEncoder.encode(response)
        }
    }
}
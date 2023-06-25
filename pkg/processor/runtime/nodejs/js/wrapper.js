/*
Copyright 2023 The Nuclio Authors.

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

const net = require('net')
const events = require('events')

const jsonCtype = 'application/json'
const initContextFunctionName = 'initContext'

const messageTypes = {
    LOG: 'l',
    RESPONSE: 'r',
    METRIC: 'm',
    START: 's',
}

const logLevels = {
    DEBUG: 'debug',
    INFO: 'info',
    WARNING: 'warning',
    ERROR: 'error',
}

let context = {
    userData: {},
    callback: async (handlerResponse) => {
        context._eventEmitter.emit('callback', handlerResponse)
    },
    Response: Response,
    logger: {
        error: logWithLevel(logLevels.ERROR),
        warn: logWithLevel(logLevels.WARNING),
        info: logWithLevel(logLevels.INFO),
        debug: logWithLevel(logLevels.DEBUG),
        errorWith: logWithLevel(logLevels.ERROR),
        warnWith: logWithLevel(logLevels.WARNING),
        infoWith: logWithLevel(logLevels.INFO),
        debugWith: logWithLevel(logLevels.DEBUG),
    },
    _socket: undefined,
    _eventEmitter: new events.EventEmitter(),
}

function Response(body = null,
                  headers = null,
                  contentType = 'text/plain',
                  statusCode = 200,
                  bodyEncoding = 'text') {
    this.body = body
    this.headers = headers
    this.content_type = contentType
    this.status_code = statusCode
    this.body_encoding = bodyEncoding

    if (!isString(this.body)) {
        this.body = JSON.stringify(this.body)
        this.content_type = jsonCtype
    }
}

function writeMessageToProcessor(messageType, messageContents) {
    context._socket.write(`${messageType}${messageContents}\n`)
}

function logWithLevel(level) {
    return (...args) => log(level, ...args)
}

function log(level, message, withData = {}) {
    const datetime = (new Date()).toISOString()
    const record = {
        datetime,
        level,
        message,
        with: withData,
    }
    writeMessageToProcessor(messageTypes.LOG, JSON.stringify(record))
}

function isString(obj) {
    return typeof (obj) === 'string' || (obj instanceof String)
}

// Status reply is a list of [status, content]
function isStatusReply(handlerOutput) {
    return Array.isArray(handlerOutput) &&
        handlerOutput.length === 2 &&
        typeof (handlerOutput[0]) === 'number'
}

function responseFromOutput(handlerOutput) {
    let response = {
        body: '',
        content_type: 'text/plain',
        headers: {},
        status_code: 200,
        body_encoding: 'text',
    }

    if (isString(handlerOutput) || (handlerOutput instanceof Buffer)) {
        response.body = handlerOutput
    } else if (isStatusReply(handlerOutput)) {
        response.status_code = handlerOutput[0]
        const body = handlerOutput[1]

        if (isString(body) || (body instanceof Buffer)) {
            response.body = body
        } else {
            response.body = JSON.stringify(body)
            response.content_type = jsonCtype
        }
    } else if (handlerOutput instanceof Response) {
        Object.assign(response, handlerOutput)
    } else {

        // other object
        response.body = JSON.stringify(handlerOutput)
        response.content_type = jsonCtype
    }

    if (Buffer.isBuffer(response.body)) {
        response.body = response.body.toString('base64')
        response.body_encoding = 'base64'
    }

    return response
}

function writeDuration(start, end) {
    const duration = {
        duration: Math.max(0.00000000001, (end.getTime() - start.getTime()) / 1000)
    }
    writeMessageToProcessor(messageTypes.METRIC, JSON.stringify(duration))
}

async function handleEvent(handlerFunction, incomingEvent) {
    let response = {}
    try {
        incomingEvent.body = new Buffer.from(incomingEvent['body'], 'base64')
        incomingEvent.timestamp = new Date(incomingEvent['timestamp'] * 1000)

        const start = new Date()

        // listening on response before executing, to avoid deadlock
        const responseWaiter = new Promise(resolve => context
            ._eventEmitter
            .once('callback', resolve))

        // call the handler
        handlerFunction(context, incomingEvent)

        // wait for callback
        const handlerResponse = await responseWaiter

        // write execution duration
        const end = new Date()
        writeDuration(start, end)
        response = responseFromOutput(handlerResponse)

    } catch (err) {
        console.log(`ERROR: ${err}`)
        let errorMessage = err.toString()

        if (err.stack !== undefined) {
            console.log(err.stack)
            errorMessage += `\n${err.stack}`
        }

        response = {
            body: `Error in handler: ${errorMessage}`,
            content_type: 'text/plain',
            headers: {},
            status_code: 500,
            body_encoding: 'text'
        }
    } finally {

        // write response
        writeMessageToProcessor(messageTypes.RESPONSE, JSON.stringify(response))
    }
}

function connectSocket(socketPath, handlerFunction) {
    const socket = new net.Socket()
    console.log(`socketPath = ${socketPath}`)
    if (socketPath.includes(':')) {

        // TCP - host:port
        const [host, portStr] = socketPath.split(':')
        const port = Number.parseInt(portStr)
        socket.connect(port, host)
    } else {

        // UNIX
        socket.connect(socketPath)
    }
    context._socket = socket
    socket.on('ready', () => {
        writeMessageToProcessor(messageTypes.START, '')
    })
    socket.on('data', async data => {
        let incomingEvent = JSON.parse(data)
        await handleEvent(handlerFunction, incomingEvent)
    })
}

function executeInitContext(functionModule) {
    const initContextFunction = functionModule[initContextFunctionName]
    if (typeof initContextFunction === 'function') {
        return initContextFunction(context)
    }
}

function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms))
}

async function findFunction(functionModule, name) {

    // attempt to find the handler for a few seconds - connect if/when we do
    // if the handler wasn't found within the limit set here, give up
    let functionToFind = functionModule[name]
    let attempts = 0
    const delayMilliseconds = 250
    while (functionToFind === undefined) {
        if (attempts < 40) {
            console.warn(`function "${name}" not found yet in ${functionModule}`)
            attempts++
            await sleep(delayMilliseconds)
        } else {
            console.warn(`function "${name}" not found by deadline in ${functionModule}`)
            throw `Failed to find function "${name}" in "${functionModule}"`
        }
        functionToFind = functionModule[name]
    }
    return functionToFind
}

function run(socketPath, handlerPath, handlerName) {
    const functionModule = require(handlerPath)
    return findFunction(functionModule, handlerName)
        .then(async handlerFunction => {
            try {
                executeInitContext(functionModule)
            } catch (err) {
                console.error(`Failed to init context: ${err}`)
                throw err
            }
            return connectSocket(socketPath, handlerFunction)
        })
}

if (require.main === module) {

    // First two arguments are ['node', '/path/to/wrapper.js']
    const args = process.argv.slice(2)

    // ['/path/to/socket', '/path/to/handler.js', 'handler']
    if (args.length !== 3) {
        console.error('error: wrong number of arguments')
        process.exit(1)
    }

    const socketPath = args[0]
    const handlerPath = args[1]
    const handlerName = args[2]

    run(socketPath, handlerPath, handlerName)
        .catch((err) => {
            console.error('Error occurred during running. Error:', err)
            process.exit(1)
        })
}

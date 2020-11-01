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

const net = require('net')
const events = require('events')

const jsonCtype = 'application/json'
const initContextFunctionName = 'initContext'

let context = {
    userData: {},
    callback: async (handlerResponse) => {
        context._eventEmitter.emit('callback', handlerResponse)
    },
    Response: Response,
    logger: {
        error: (message, withData) => log('error', message, withData),
        warn: (message, withData) => log('warning', message, withData),
        info: (message, withData) => log('info', message, withData),
        debug: (message, withData) => log('debug', message, withData),
        errorWith: (message, withData) => log('error', message, withData),
        warnWith: (message, withData) => log('warning', message, withData),
        infoWith: (message, withData) => log('info', message, withData),
        debugWith: (message, withData) => log('debug', message, withData),
    },
    _socket: undefined,
    _eventEmitter: new events.EventEmitter(),
}

function Response(body = null,
                  headers = null,
                  contentType = 'text/plain',
                  statusCode = 200) {
    this.body = body
    this.headers = headers
    this.content_type = contentType
    this.status_code = statusCode

    if (!isString(this.body)) {
        this.body = JSON.stringify(this.body)
        this.content_type = jsonCtype
    }
}

function log(level, message, withData) {
    if (withData === undefined) {
        withData = {}
    }

    const record = {
        datetime: new Date().toISOString(),
        level: level,
        message: message,
        with: withData,
    }
    context._socket.write('l' + JSON.stringify(record) + '\n')
}

function isString(obj) {
    return typeof (obj) === 'string' || (obj instanceof String)
}

// Status reply is a list of [status, content]
function isStatusReply(handlerOutput) {
    return handlerOutput instanceof Array &&
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
        response.body = handlerOutput.body
        response.content_type = handlerOutput.content_type
        response.headers = handlerOutput.headers
        response.status_code = handlerOutput.status_code
    } else {

        // other object
        response.body = JSON.stringify(handlerOutput)
        response.content_type = jsonCtype
    }

    if (response.body instanceof Buffer) {
        response.body = response.body.toString('base64')
        response.body_encoding = 'base64'
    }

    return response
}

function writeDuration(start, end) {
    const duration = {
        duration: Math.max(0.00000000001, (end.getTime() - start.getTime()) / 1000)
    }
    context._socket.write('m' + JSON.stringify(duration) + '\n')
}

async function handleEvent(handlerFunction, incomingEvent) {
    try {
        incomingEvent.body = new Buffer.from(incomingEvent['body'], 'base64')
        incomingEvent.timestamp = new Date(incomingEvent['timestamp'] * 1000)

        const start = new Date()

        // listening on response before executing, to avoid deadlock
        const responseWaiter = new Promise(resolve => context
            ._eventEmitter
            .on('callback', resolve))

        // call the handler
        handlerFunction(context, incomingEvent)

        // wait for callback
        const handlerResponse = await responseWaiter

        // write execution duration
        const end = new Date()
        writeDuration(start, end)

        // write response
        const response = responseFromOutput(handlerResponse)
        context._socket.write('r' + JSON.stringify(response) + '\n')
    } catch (err) {
        console.log('ERROR: ' + err)
        let errorMessage = err.toString()

        if (err.stack !== undefined) {
            console.log(err.stack)
            errorMessage += '\n' + err.stack
        }

        const response = {
            body: 'Error in handler: ' + errorMessage,
            content_type: 'text/plain',
            headers: {},
            status_code: 500,
            body_encoding: 'text'
        }

        context._socket.write('r' + JSON.stringify(response) + '\n')
    }
}

function connectSocket(socketPath, handlerFunction) {
    const socket = new net.Socket()
    console.log('socketPath = ' + socketPath)
    if (/:/.test(socketPath)) {

        // TCP - host:port
        const parts = socketPath.split(':')
        const host = parts[1]
        const port = parseInt(parts[0])

        socket.connect(port, host)
    } else {

        // UNIX
        socket.connect(socketPath)
    }
    context._socket = socket
    socket.on('data', async data => {
        let incomingEvent = JSON.parse(data)
        await handleEvent(handlerFunction, incomingEvent)
    })
}

function executeInitContext(functionModule) {
    const initContextFunction = functionModule[initContextFunctionName]
    if (initContextFunction !== undefined) {
        initContextFunction(context)
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
            console.warn('function "' + name + '" not found yet in ' + functionModule)
            attempts++
            await sleep(delayMilliseconds)
        } else {
            console.error('error: function "' + name + '" not found by deadline in ' + functionModule)
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
                console.error('Failed to init context: ' + err)
                throw err
            }
            return connectSocket(socketPath, handlerFunction)
        })
        .catch(err => {
            console.error('Failed to to find function and run it ' + err)
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
}

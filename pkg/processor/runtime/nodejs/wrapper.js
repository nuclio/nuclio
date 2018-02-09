net = require('net');

var json_ctype = 'application/json';

function Response(body=null, headers=null, content_type='text/plain', status_code=200) {
    this.body = body;
    this.headers = headers;
    this.content_type = content_type;
    this.status_code = status_code;

    if (!is_string(this.body)) {
	this.body = JSON.stringify(this.body);
	this.content_type = json_ctype;
    }
}

function log(level, message, with_data) {
    if (with_data === undefined) {
	with_data = {}
    }

    var record = {
	datetime: new Date().toISOString(),
	level: level,
	message: message,
	'with': with_data
    }
    socket.write('l' + JSON.stringify(record) + '\n');
}

function is_string(obj) {
    return typeof(obj) == 'string' || (obj instanceof String)
}

// Status reply is a list of [status, content]
function is_status_reply(handler_output) {
    if (!handler_output instanceof Array) {
	return false;
    }

    if (handler_output.length != 2) {
	return false;
    }

    if (typeof(handler_output[0]) != 'number') {
	return false;
    }

    return true;
}

function response_from_output(handler_output) {
    var response = {
        body: '',
        content_type: 'text/plain',
        headers: {},
        status_code: 200,
        body_encoding: 'text'
    };

    if (is_string(handler_output) || (handler_output instanceof Buffer)) {
        response.body = handler_output;
    } else if (is_status_reply(handler_output)) {
	response.status_code = handler_output[0]
	var body = handler_output[1];

	if (is_string(body) || (body instanceof Buffer)) {
	    response.body = body;
	} else {
	    response.body = JSON.stringify(body);
	    response.content_type = json_ctype;
	}
    } else if (handler_output instanceof Response) {
        response.body = handler_output.body;
        response.content_type = handler_output.content_type;
        response.headers = handler_output.headers;
        response.status_code = handler_output.status_code;
    } else { // other object
	response.body = JSON.stringify(handler_output);
	response.content_type = json_ctype;
    }

    if (response.body instanceof Buffer) {
	response.body = response.body.toString('base64');
	response.body_encoding = 'base64';
    }

    return response;
}

function send_reply(handler_output) {
    var response = response_from_output(handler_output);
    socket.write('r' + JSON.stringify(response) + '\n');
}

var context = {
    callback: send_reply,
    Response: Response,

    logger: {
	error: function(message) { log('error', message); },
	warn: function(message) { log('warning', message);},
	info: function(message) { log('info', message);},
	debug: function(message) { log('debug', message);},
	errorWith: function(message, with_data) { log('error', message, with_data); },
	warnWith: function(message, with_data) { log('warning', message, with_data);},
	infoWith: function(message, with_data) { log('info', message, with_data);},
	debugWith: function(message, with_data) { log('debug', message, with_data);},
    }
};

function connectSocket() {
    var conn = socketPath;
    console.log('conn = ' + conn);
    if (/:/.test(conn)) { // TCP - host:port
        var parts = conn.split(':')
        var host = parts[1];
        var port = parseInt(parts[0]);

        socket.connect(port, host);
    } else { // UNIX
        socket.connect(conn);
    }

    socket.on('data', function(data) {
        try {
            var evt = JSON.parse(data);
            evt.body = new Buffer(evt.body, 'base64');
            evt.timestamp = new Date(evt['timestamp'] * 1000);

            // call the handler
            handlerFunc(context, evt);
        } catch (err) {
            console.log('ERROR: ' + err);
            var error_message = err.toString();

            if (err.stack !== undefined) {
                console.log(err.stack);
                error_message += '\n' + err.stack;
            }

            var response = {
                body: 'Error in handler: ' + error_message,
                content_type: 'text/plain',
                headers: {},
                status_code: 500,
                body_encoding: 'text'
            };

            socket.write('r' + JSON.stringify(response) + '\n');
        }
    });
}

if (require.main === module) {

    // First two arguments are ['node', '/path/to/wrapper.js']
    var args = process.argv.slice(2);

    // ['/path/to/socket', '/path/to/handler.js', 'handler']
    if (args.length != 3) {
	console.error('error: wrong number of arguments');
	process.exit(1);
    }

    var socketPath = args[0];
    var handlerPath = args[1];
    var handlerName = args[2];

    var module = require(handlerPath);
    var socket = new net.Socket();

    // attempt to find the handler for a few seconds - connect if/when we do
    // if the handler wasn't found within the limit set here, give up
    var handlerFunc = undefined;
    var findHandlerAttempts = 0;
    var delayMilliseconds = 250;

    function sleep(ms) {
        return new Promise(resolve => setTimeout(resolve, ms))
    }

    async function findHandler() {
        while (handlerFunc === undefined) {
            handlerFunc = module[handlerName];

            if (handlerFunc === undefined) {
                if (findHandlerAttempts < 40) {
                    console.warn('handler "' + handlerName + '" not found yet in ' + handlerPath);
                    findHandlerAttempts++;
                    await sleep(delayMilliseconds);
                } else {
                    console.error('error: handler "' + handlerName + '" not found by deadline in ' + handlerPath);
                }
            }
        }

        console.debug('found handler, connecting to socket')
        connectSocket()
    }

    findHandler()
}

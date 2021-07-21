import {
    ConnLogger,
    Handler,
    JSONConn,
    MESSAGE_TYPE,
    ReqEvent,
} from "https://cdn.jsdelivr.net/npm/nuclio-deno@1.0.4/mod.ts";

import { assert } from "https://deno.land/std@0.86.0/testing/asserts.ts";

const [socketPath, handlerPath] = Deno.args;

assert(typeof socketPath === 'string', 'Invalid socket path');
assert(typeof handlerPath === 'string', 'Invalid handler path');

console.log('Importing handler');

let handlerConstructor: new () => Handler<any>;
try {
    handlerConstructor = (await import(handlerPath)).default;
} catch (e) {
    throw new Error(`Failed to import script: ${e.message}`);
}

console.log('Handler imported');
console.log('Awaiting connection');

console.log(handlerConstructor);


for await(const conn of JSONConn.listen(socketPath)) {
    console.log('New connection received');
    const handler = new handlerConstructor();
    const logger = new ConnLogger(conn.conn);
    const context = {logger};

    conn.conn.writeWithHeader(MESSAGE_TYPE.START, '');
    logger.info('Started');
    try {
        for await(const msg of conn) {
            try {
                logger.debug('New message', {msg});

                const event: ReqEvent<any> = msg;

                const start = Date.now();
                const res = await handler.run(context, event);
                const end = Date.now();

                const duration = {
                    duration: Math.max(0.00000000001, (end - start) / 1000)
                }

                conn.writeWithHeader(MESSAGE_TYPE.METRIC, duration);

                conn.writeWithHeader(MESSAGE_TYPE.RESPONSE, res);
            } catch (e) {
                const response = {
                    body: `Error in handler: ${e.message}`,
                    content_type: 'text/plain',
                    headers: {},
                    status_code: 500,
                    body_encoding: 'text'
                }

                conn.writeWithHeader(MESSAGE_TYPE.RESPONSE, response);
            }
        }
    } catch (e) {
        conn.close();
        console.error(e);
    }
}
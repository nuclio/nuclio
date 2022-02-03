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

const assert = require('assert')
const net = require('net')
const fs = require('fs')
const rewire = require('rewire')
const wrapper = rewire('./wrapper.js')

const projectRoot = (process.env.RUN_MODE === 'CI') ? '..' : '../../../../..'
const testFunctionsDirPath = `${projectRoot}/test/_functions`

describe('Wrapper', () => {
    describe('findFunction()', () => {
        it('should find function handler', async function () {
            const functionModulePath = `${testFunctionsDirPath}/common/reverser/nodejs/handler.js`
            const functionModule = require(functionModulePath)
            const foundFunction = await wrapper.__get__('findFunction')(
                functionModule,
                'handler'
            )
            assert.strictEqual(foundFunction, functionModule.handler)
        })
    })
    describe('context.logger.<level>()', () => {
        it('should log with level', function () {
            const context = wrapper.__get__('context')
            let writtenData = ''
            context._socket = {
                write: (message) => {
                    writtenData = message
                }
            }
            context.logger.info('HelloWorld', { a: 2 })
            const writtenAsObject = JSON.parse(writtenData.substring(1))

            // it is not empty
            assert.notStrictEqual(writtenAsObject.datatime, '')
            assert.strictEqual(writtenAsObject.level, 'info')
            assert.strictEqual(writtenAsObject.message, 'HelloWorld')
            assert.deepStrictEqual(writtenAsObject.with, { a: 2 })
        })
    })
    describe('handleEvent()', () => {
        it('should response with output', async () => {
            const functionModulePath = `${testFunctionsDirPath}/common/reverser/nodejs/handler.js`
            const functionModule = require(functionModulePath)
            const handlerFunction = await wrapper.__get__('findFunction')(
                functionModule,
                'handler'
            )
            const context = wrapper.__get__('context')
            const handleEvent = wrapper.__get__('handleEvent')
            const writtenData = []
            context._socket = {
                write: (message) => {
                    writtenData.push(message)
                }
            }
            const event = { body: Buffer.from('abc').toString('base64') }
            await handleEvent(handlerFunction, event)
            const responseData = JSON.parse(writtenData[1].substring(1))
            assert.strictEqual(responseData.body, 'cba')
        })
        it('should remove callback listener from context event emitter', async () => {
            const functionModulePath = `${testFunctionsDirPath}/common/reverser/nodejs/handler.js`
            const functionModule = require(functionModulePath)
            const context = wrapper.__get__('context')
            const handleEvent = wrapper.__get__('handleEvent')
            const writtenData = []
            context._socket = {
                write: (message) => {
                    writtenData.push(message)
                }
            }
            const handlerFunction = await wrapper.__get__('findFunction')(
                functionModule,
                'handler'
            )

            // create some requests
            const promises = []
            for (let i = 0; i < 1000; i++) {
                promises.push(handleEvent(handlerFunction, {
                    body: Buffer.from('abc' + i).toString('base64'),
                }))
            }

            // handle all requests
            await Promise.all(promises)

            // each handled event sends both "response" and "metric"
            assert.strictEqual(promises.length, writtenData.length / 2)

            // all callbacks listeners were closed
            assert.strictEqual(context._eventEmitter.listenerCount('callback'), 0)
        })
    })
    describe('initContext()', () => {
        it('should mutate context object', async () => {
            const functionModulePath = `${testFunctionsDirPath}/common/context-init/nodejs/contextinit.js`
            const functionModule = require(functionModulePath)
            const context = wrapper.__get__('context')
            const executeInitContext = wrapper.__get__('executeInitContext')
            executeInitContext(functionModule)
            assert.strictEqual(context.userData.factor, 2)
        })
        it('should skip initContext when function not exposed', async () => {
            const functionModulePath = `${testFunctionsDirPath}/common/reverser/nodejs/handler.js`
            const functionModule = require(functionModulePath)
            const executeInitContext = wrapper.__get__('executeInitContext')
            try {
                executeInitContext(functionModule)
            } catch (err) {
                assert.fail(`InitContext should be skipped of \`initContext\` function is not exposed. err: ${err}`)
            }
        })
        it('should fail executing initContext', () => {
            const functionModulePath = `${testFunctionsDirPath}/common/context-init-fail/nodejs/contextinitfail.js`
            const functionModule = require(functionModulePath)
            const executeInitContext = wrapper.__get__('executeInitContext')
            assert.throws(() => {
                executeInitContext(functionModule)
            }, Error)
        })
    })
    describe('run()', function () {
        const socketPath = '/tmp/just-a-socket'
        it('should run wrapper', function (done) {
            const handlerPath = `${testFunctionsDirPath}/common/context-init/nodejs/contextinit.js`
            const handlerName = 'handler'
            const run = wrapper.__get__('run')
            let responses = []
            const server = net.createServer(socket => {
                const number = 10

                // set in function initContext
                const factor = 2
                const requestBody = {
                    body: (new Buffer.from(number.toString())).toString('base64')
                }
                socket.write(new Buffer.from(JSON.stringify(requestBody)))
                socket.on('data', data => {
                    if (data.toString().trim() === 's') {

                        // ignore start message
                        return
                    }
                    responses = [
                        ...responses,
                        ...data.toString().trim().split('\n'),
                    ].filter(response => response !== 's').map(response => response.substring(1))
                    socket.end()
                    server.close()
                    assert.strictEqual(JSON.parse(responses[1]).body, (number * factor).toString())
                    done()
                })
            })
            server.listen(socketPath)
            run(server.address(), handlerPath, handlerName)
        })
        after(() => {
            if (fs.existsSync(socketPath)) {
                fs.unlinkSync(socketPath)
            }
        })
    })
})

var fs = require('fs');
var express = require('express');

var app = express();

module.exports = function () {

    /**
     * Generates mock request
     * @param requestMock
     */
    var mockRequest = function (requestMock) {
        app[requestMock.method.toLowerCase()](requestMock.url, requestMock.handler);
    };

    /**
     * Starts mock server
     * @param done
     */
    var start = function (done) {
        this.server = app.listen(8081, null, null, done);
    };

    /**
     * Stops mock server
     */
    var stop = function () {
        this.server.close();
    };

    return {
        startServer: start,
        stopServer: stop,
        mockRequest: mockRequest
    }
};

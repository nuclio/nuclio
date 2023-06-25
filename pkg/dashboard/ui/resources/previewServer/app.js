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

var express = require('express');
var serveStatic = require('serve-static');
var proxy = require('express-http-proxy');
var path = require('path');
var _ = require('lodash');

var app = express();

var previewServer = function () {
    var start = function (log, rootPath) {
        var root = path.resolve(__dirname, '../../' + rootPath);
        var backendPort = _.defaultTo(process.env.NCL_BACKEND_LISTEN_PORT, 8070);

        // if a request sent to /api/path?query redirect to http://127.0.0.1:8070/api/path?query
        app.use('/api', proxy('http://127.0.0.1:' + backendPort, {
            limit: '15mb',
            proxyReqPathResolver: function (request) {
                return request.originalUrl;
            }
        }));

        app.use(serveStatic(root, {
            maxAge: '1d',
            setHeaders: setCustomCacheControl
        }));

        app.all('*', function (req, res) {
            res.sendFile(root + '/index.html');
        });

        var port = process.env.NCL_PREVIEW_LISTEN_PORT || 8000;

        this.server = app.listen(port, function () {
            log('Preview server listening on ' + port);
        });

        function setCustomCacheControl(res, path) {
            var mimeTypesWithNoCaching = ['text/html', 'application/javascript'];
            if (_.includes(mimeTypesWithNoCaching, serveStatic.mime.lookup(path))) {

                // Custom Cache-Control for HTML files
                res.setHeader('Cache-Control', 'max-age=0, must-revalidate');
            }
        }
    };

    var stop = function () {
        this.server.close();
    };

    return {
        start: start,
        stop: stop
    }
}();

module.exports = previewServer;

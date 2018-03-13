var express = require('express');
var serveStatic = require('serve-static');
var path = require('path');
var _ = require('lodash');

var app = express();

var previewServer = function () {
    var start = function (log, rootPath) {
        app.use(serveStatic(path.resolve(__dirname, '../../' + rootPath), {
            maxAge: '1d',
            setHeaders: setCustomCacheControl
        }));
        app.all('*', function (req, res) {
            res.sendFile(path.resolve(__dirname, '../../' + rootPath + '/index.html'));
        });

        var port = process.env.IGZ_PREVIEW_LISTEN_PORT || 8000;

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

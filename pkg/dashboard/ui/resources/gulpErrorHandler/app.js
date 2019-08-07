/**
 * Error handling object
 */

var color = require('ansi-colors');
var log = require('fancy-log');

var errHandler = new (function () {
    var self = this;
    var mode = 'terminate';
    var uses = [];

    this.resist = function () {
        mode = 'pass';
        log('Gulp error handler installed');
    };

    this.use = function (fn) {
        if (typeof fn !== 'function') {

            console.error('errHandler.use expects a function argument');
            return;
        }

        uses.push(fn);
    };

    this.swallow = function ($ident) {
        return function (err) {
            if (mode == 'pass') {
                log(
                    color.red('An error occured with ('),
                    color.yellow($ident),
                    color.red('), but was resisted.')
                );

                this.emit('end');
            } else {
                uses.forEach(function (fn) {
                   fn(err);
                });

                this.emit('error');
            }
        };
    };
})();

module.exports = errHandler;

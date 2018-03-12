/**
 * Error handling object
 */

var gutil = require('gulp-util');

var errHandler = new (function () {
    var self = this;
    var mode = 'terminate';
    var uses = [];

    this.resist = function () {
        mode = 'pass';
        gutil.log('Gulp error handler installed');
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
                gutil.log(
                    gutil.colors.red('An error occured with ('),
                    gutil.colors.yellow($ident),
                    gutil.colors.red('), but was resisted.')
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
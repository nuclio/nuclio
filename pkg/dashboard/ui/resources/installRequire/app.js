/**
 * A synchronous module which is used to make sure that imported modules have their
 * dependencies installed.
 *
 * In order to maintain the module as dependency-free, it requires one of the two:
 *
 * 1. The nodejs version to be 0.12.0 and above; the child_process module has a synchronous shell execution
 *    method in that version;
 * 2. the sync-exec module to be injected by invoking the setup method
 *
 * This module is SYNCHRONOUS, meaning that it WILL block execution of code, and that is the exact purpose:
 * to wrap itself around the 'require' keyword and allow npm to install dependencies prior to invoking it.
 */

var fs = require('fs');
var childProcess = require('child_process');
var execSync = childProcess.execSync;
var gutil = console;

function _checkDependenciesStatus(src) {
    return fs.existsSync(src + '/node_modules') ? 'ok' : 'missing';
}

function _installDependencies(src) {
    gutil.log('Installing node dependencies for', src);
    try {
        var commands = ['cd ' + src, 'npm install --silent'];
        execSync(commands.join(' && '));
    } catch (e) {
        gutil.log(
            'An error has occurred during dependency installation for',
            src,
            '; reason:',
            e
        );

        try {
            fs.rmdirSync(src + '/node_modules');
        } catch (ignored) {}

        throw 'Unable to install dependencies for module ' + src;
    }
}

/**
 * Installs the given module's dependencies, then requires it.
 * @param {string} src  the source path of the intended module to import
 * @returns the required module
 */
module.exports = function installRequire(src) {
    var realPathSrc = fs.realpathSync(src);

    if (_checkDependenciesStatus(realPathSrc) === 'missing') {
        _installDependencies(realPathSrc);
    }

    return require(fs.realpathSync(realPathSrc));
};

/**
 * Sets up and configs the modules used.
 * @param [execSyncModule]  sync-exec module {@link http://npmjs.com/package/sync-exec}
 *                          can be null if using nodejs v0.12.x
 * @param [gutilModule]     gutil module {@link http://npmjs.com/package/gutil}
 *                          if null, will use console instead
 */
module.exports.setup = function setup(execSyncModule, gutilModule) {
    execSync = childProcess.execSync || execSyncModule;
    gutil = gutilModule || console;
};
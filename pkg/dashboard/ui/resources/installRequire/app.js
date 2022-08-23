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

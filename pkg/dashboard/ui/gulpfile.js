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

//
// ******* Configuration and loading third party components *******
//

/**
 * Load required components
 */

// Do not put here required modules that are in devDependencies in package.json, instead require them only in the
// specific gulp task that uses them (for example: karma, protractor, livereload)
var babel = require('gulp-babel');
var color = require('ansi-colors');
var config = require('./build.config');
var cache = require('gulp-file-transform-cache');
var gulp = require('gulp');
var path = require('path');
var less = require('gulp-less');
var lessImport = require('gulp-less-import');
var log = require('fancy-log');
var rename = require('gulp-rename');
var concat = require('gulp-concat');
var eslint = require('gulp-eslint');
var preprocess = require('gulp-preprocess');
var minifyCss = require('gulp-clean-css');
var gulpIf = require('gulp-if');
var rev = require('gulp-rev');
var argv = require('yargs').argv;
var minifyHtml = require('gulp-htmlmin');
var ngHtml2Js = require('gulp-ng-html2js');
var merge2 = require('merge2');
var uglify = require('gulp-uglify');
var revCollector = require('gulp-rev-collector');
var imagemin = require('gulp-imagemin');
var iRequire = require('./resources/installRequire');
var lodash = require('lodash');
var del = require('del');
var vinylPaths = require('vinyl-paths');
var exec = require('child_process').exec;
var errorHandler = require('gulp-error-handle');
var buildVersion = null;
var livereload = null;

/**
 * Set up configuration
 */
var state = {
    isDevMode: argv.dev === true, // works only for development build type
    isForTesting: false,
    isForE2ETesting: false
};

/**
 * Load components for development environment
 */
if (state.isDevMode) {
    livereload = require('gulp-livereload');
}

/**
 * Make sure resources are built before app
 */
var previewServer = iRequire(config.resources.previewServer);

//
// ******* Tasks *******
//

gulp.task('build', build);
gulp.task('test-unit', testUnit);
gulp.task('test-e2e', testE2e);
gulp.task('test', test);
gulp.task('watch', watch);

//
// ******* Functions *******
//

/**
 * Set build for testing
 */
function setTesting(next) {
    state.isForTesting = true;
    state.isDevMode = true;

    next();
}

/**
 * Set build for testing
 */
function setE2eTesting() {
    state.isForE2ETesting = true;
    //state.isDevMode = true;
}

/**
 * Clean build directory
 */
function clean() {
    return gulp.src([config.build_dir, config.cache_file], {allowEmpty: true})
        .pipe(errorHandler(handleError))
        .pipe(vinylPaths(del));
}

/**
 * Build vendor.css (include all vendor css files)
 */
function vendorCss() {
    var distFolder = config.assets_dir + '/css';

    return merge2(
        gulp.src(config.vendor_files.less, {allowEmpty: true})
            .pipe(errorHandler(handleError))
            .pipe(lessImport('bootstrap.less'))
            .pipe(less()),
        gulp.src([path.join(distFolder, 'bootstrap.css')].concat(config.vendor_files.css), {allowEmpty: true}))
        .pipe(errorHandler(handleError))
        .pipe(concat(config.output_files.vendor.css))
        .pipe(gulpIf(!state.isDevMode, minifyCss()))
        .pipe(gulpIf(!state.isDevMode, rev()))
        .pipe(gulp.dest(distFolder))
        .pipe(gulpIf(!state.isDevMode, rev.manifest(config.output_files.vendor.css_manifest)))
        .pipe(gulp.dest(distFolder));
}

/**
 * Build vendor.js (include all vendor js files)
 */
function vendorJs() {
    var distFolder = config.assets_dir + '/js';

    return gulp.src(config.vendor_files.js, {allowEmpty: true})
        .pipe(errorHandler(handleError))
        .pipe(concat(config.output_files.vendor.js))
        .pipe(gulpIf(!state.isDevMode, uglify()))
        .pipe(gulpIf(!state.isDevMode, rev()))
        .pipe(gulp.dest(distFolder))
        .pipe(gulpIf(!state.isDevMode, rev.manifest(config.output_files.vendor.js_manifest)))
        .pipe(gulp.dest(distFolder));
}

/**
 * Build app.css (include all project css files)
 */
function appCss() {
    var distFolder = config.assets_dir + '/css';

    var task = gulp
        .src(config.app_files.less_files)
        .pipe(errorHandler(handleError))
        .pipe(lessImport('app.less'))
        .pipe(less({
            paths: [path.join(__dirname, 'less', 'includes')],
            compress: false
        }))
        .pipe(less({
            compress: !state.isDevMode
        }))
        .pipe(rename(config.output_files.app.css))
        .pipe(gulpIf(!state.isDevMode, rev()))
        .pipe(gulp.dest(distFolder))
        .pipe(gulpIf(!state.isDevMode, rev.manifest(config.output_files.app.css_manifest)))
        .pipe(gulp.dest(distFolder));

    if (livereload !== null) {
        task.pipe(livereload());
    }

    return task;
}

/**
 * Build app.js (include all project js files and templates)
 */
function appJs() {
    var distFolder = config.assets_dir + '/js';
    var customConfig = buildConfigFromArgs();

    var js = gulp.src(config.app_files.js, {allowEmpty: true})
        .pipe(errorHandler(handleError))
        .pipe(preprocess({
            context: {
                IGZ_CUSTOM_CONFIG: customConfig || '',
                IGZ_TESTING: state.isForTesting,
                IGZ_E2E_TESTING: state.isForE2ETesting,
                IGZ_DEVELOPMENT_BUILD: state.isDevMode
            }
        }))
        .pipe(cache({
            path: config.cache_file,
            transformStreams: [
                babel({
                    ignore: ['node_modules/iguazio.dashboard-controls/dist/js/iguazio.dashboard-controls.js']
                })
            ]
        }));

    var templates = gulp.src(config.app_files.templates, {allowEmpty: true})
        .pipe(errorHandler(handleError))
        .pipe(minifyHtml({
            removeComments: true,
            collapseWhitespace: true,
            collapseInlineTagWhitespace: true,
            conservativeCollapse: true
        }))
        .pipe(ngHtml2Js({
            moduleName: config.app_files.templates_module_name
        }));

    var task;

    if (state.isForTesting) {
        task = merge2(js, templates)
            .pipe(errorHandler(handleError))
            .pipe(concat(config.output_files.app.js))
            .pipe(gulp.dest(distFolder));
    } else {
        task = merge2(js, templates)
            .pipe(errorHandler(handleError))
            .pipe(concat(config.output_files.app.js))
            .pipe(gulpIf(!state.isDevMode, rev()))
            .pipe(gulp.dest(distFolder))
            .pipe(gulpIf(!state.isDevMode, rev.manifest(config.output_files.app.js_manifest)))
            .pipe(gulp.dest(distFolder));
    }

    if (state.isDevMode && livereload !== null) {
        task.pipe(livereload());
    }

    return task;
}

/**
 * Temporary task to copy the monaco-editor files to the assets directory
 */
function monaco(next) {
    gulp.src(['node_modules/monaco-editor/**/*'], {allowEmpty: true})
        .pipe(gulp.dest(config.assets_dir + '/monaco-editor'));
    next();
}

/**
 * Copy all fonts to the build directory
 */
function fonts() {
    var distFolder = config.assets_dir + '/fonts';

    return gulp.src(config.app_files.fonts + '/**/*', {allowEmpty: true})
        .pipe(errorHandler(handleError))
        .pipe(gulp.dest(distFolder));
}

/**
 * Optimize all images and copy them to the build directory
 */
function images() {
    var distFolder = config.assets_dir + '/images';

    return gulp.src(config.app_files.images, {allowEmpty: true})
        .pipe(errorHandler(handleError))
        .pipe(gulpIf(!state.isDevMode, imagemin({
            optimizationLevel: 3,
            progressive: true,
            interlaced: true
        })))
        .pipe(gulp.dest(distFolder));
}

/**
 * Copy all translation files to the build directory
 */
function i18n() {
    var distFolder = config.assets_dir + '/i18n';

    return gulp.src(config.app_files.i18n, {allowEmpty: true})
        .pipe(errorHandler(handleError))
        .pipe(gulp.dest(distFolder));
}

/**
 * Build index.html for ordinary use
 */
function indexHtml() {
    return buildIndexHtml(false);
}

/**
 * Build dashboard-config.json
 */
function dashboardConfigJson() {
    return gulp.src(config.app_files.json, {allowEmpty: true})
        .pipe(errorHandler(handleError))
        .pipe(gulp.dest(config.build_dir));
}

/**
 * Lint source code
 */
function lint() {
    return gulp.src(config.app_files.js, {allowEmpty: true})
        .pipe(errorHandler(handleError))
        .pipe(eslint())
        .pipe(eslint.format('compact'))
        .pipe(eslint.failAfterError());
}

/**
 * Serve static files
 */
function serveStatic(next) {
    previewServer.start(log, config.build_dir);
    next();
}

/**
 * Run unit tests (Karma)
 * Task for development environment only
 */
function testUnitRun(done) {
    var KarmaServer = require('karma').Server;
    var files = [__dirname + '/' + config.assets_dir + '/js/' + config.output_files.vendor.js]
        .concat(__dirname + '/' + config.test_files.unit.vendor)
        .concat([__dirname + '/' + config.assets_dir + '/js/' + config.output_files.app.js])
        .concat(__dirname + '/' + (!lodash.isUndefined(argv.spec) ? 'src/**/' + argv.spec : config.test_files.unit.tests));

    new KarmaServer({
        configFile: __dirname + '/' + config.test_files.unit.karma_config,
        files: files,
        action: 'run'
    }, done).start();
}

/**
 * Build e2e mock module with dependencies
 */
function testE2eMockModule() {
    var files = config.test_files.e2e.vendor
        .concat(config.test_files.e2e.mock_module);

    return gulp.src(files, {allowEmpty: true})
        .pipe(errorHandler(handleError))
        .pipe(concat(config.test_files.e2e.built_file_name))
        .pipe(gulp.dest(config.test_files.e2e.built_folder_name));
}

/**
 * Process index.html and inject mocked module for e2e testing
 */
function testE2eMockHtml() {
    return buildIndexHtml(true);
}

/**
 * Print info about test-e2e-run task options
 * Task for development environment only
 */
function e2eHelp(next) {
    var greenColor = '\x1b[32m';
    var regularColor = '\x1b[0m';
    var helpMessage = '\n' +
        greenColor + '--browsers={number}' + regularColor + '\n\toption for setting count of browser instances to run\n' +
        greenColor + '--run-single' + regularColor + '\n\toption for running all specs in one thread\n' +
        greenColor + '--specs={string}' + regularColor + '\n\tcomma separated set of specs for test run.\n\tSee: ./build.config -> test_files.e2e.spec_path\n' +
        greenColor + '--spec-pattern={string}' + regularColor + '\n\tcomma separated set of spec patterns for including to test run\n' +
        greenColor + '--exclude-pattern={string}' + regularColor + '\n\tcomma separated set of spec patterns for excluding from test run\n' +
        greenColor + '--junit-report' + regularColor + '\n\toption for generating test reporter in XML format that is compatible with JUnit\n' +
        greenColor + '--dont-update-wd' + regularColor + '\n\toption to prevent WebDriver updating';
    next();
    console.info(helpMessage);
}

/**
 * Run e2e tests (Protractor)
 * Task for development environment only
 */
function testE2eRun() {
    console.info('Use \'gulp e2e-help\' to get info about test run options');
    var argumentList = [];
    var src = [];
    var browserInstances = 3;
    var exclusions = [];
    var protractor = require('gulp-protractor').protractor;

    /**
     * --browsers={number} - option for setting count of browser instances to run
     * @type {number}
     */
    if (argv['browsers']) {
        browserInstances = parseInt(argv['browsers']);
    }

    if (argv['demo']) {
        argumentList.push(
            '--params.use_mode=demo'
        );
    } else {
        argumentList.push(
            '--params.use_mode=staging'
        );
    }

    /**
     * --run-single - option for running all specs in one thread
     */
    if (!argv['run-single']) {
        argumentList.push(
            '--capabilities.maxInstances', browserInstances,
            '--capabilities.shardTestFiles', true
        );
    }

    /**
     * --specs={string} - comma separated list of specs for test run.
     * See: ./build.config -> test_files.e2e.spec_path
     * @type {string}
     */
    if (argv.specs) {
        argv.specs.split(',').forEach(function (specArgument) {
            src.push(config.test_files.e2e.spec_path[specArgument.trim()]);
        });
    }

    /**
     * --spec-pattern={string} - comma separated list of spec patterns for including to test run
     * @type {string}
     */
    if (argv['spec-pattern']) {
        argv['spec-pattern'].split(',').forEach(function (specPattern) {
            src.push(config.test_files.e2e.specs_location + specPattern.trim() + '.spec.js');
        });
        console.info('Ran specs:\n' + src.join(',\n'));
    }

    /**
     * --exclude-pattern={string} - comma separated list of spec patterns for excluding from test run
     * @type {string}
     */
    if (argv['exclude-pattern']) {
        argv['exclude-pattern'].split(',').forEach(function (excludePattern) {
            exclusions.push(config.test_files.e2e.specs_location + excludePattern.trim() + '.spec.js');
        });
        argumentList.push(
            '--exclude', exclusions.join(',')
        );
        console.info('Excluded specs:\n' + exclusions.join(',\n'));
    }

    /**
     * --junit-report - option for generating test reporter in XML format that is compatible with JUnit
     */
    if (argv['junit-report']) {
        argumentList.push(
            '--params.use_junit_reporter=true'
        );
        console.info('JUnit reporter will be used');
    }

    if (src.length === 0) {
        Object.values(config.test_files.e2e.spec_path).forEach(function (value) {
            src.push(value);
        });
    }

    return gulp.src(src, {allowEmpty: true})
        .pipe(protractor({
            configFile: config.test_files.e2e.protractor_config,
            args: argumentList
        }))
        .on('error', function (e) {
            var currentTime = new Date();
            console.error('[' + currentTime.getHours() + ':' + currentTime.getMinutes() + ':' +
                currentTime.getSeconds() + '] ');
            throw e;
        });
}

/**
 * Stop the server
 */
function stopServer(next) {
    previewServer.stop();
    next();
}

/**
 * Watch for changes and build needed sources
 * Task for development environment only
 */
function watcher(next) {
    state.isDevMode = true;
    if (livereload !== null) {
        livereload.listen();
    }

    gulp.watch(config.app_files.less_files, appCss);
    log('Watching', color.yellow('LESS'), 'files');

    var appFiles = config.app_files.js
        .concat(config.app_files.templates);
    gulp.watch(appFiles, appJs);
    log('Watching', color.yellow('JavaScript'), 'files');

    gulp.watch(config.app_files.html, indexHtml);
    log('Watching', color.yellow('HTML'), 'files');

    gulp.watch(config.app_files.json, dashboardConfigJson);
    log('Watching', color.blue('JSON'), 'files');

    gulp.watch(config.app_files.i18n, {interval: 3000}, i18n);
    log('Watching', color.blue('I18N'), 'files');

    gulp.watch(config.shared_files.less, buildShared);
    log('Watching', color.yellow('LESS'), 'shared_files');

    var appFilesShared = config.shared_files.js
        .concat(config.shared_files.templates);
    gulp.watch(appFilesShared, buildShared);
    log('Watching', color.yellow('JavaScript'), 'shared_files');

    gulp.watch(config.shared_files.i18n, {interval: 3000}, buildShared);
    log('Watching', color.blue('I18N'), 'shared_files');

    next();
}

/**
 * Update web driver
 * Task for development environment only
 */
function updateWebDriver(next) {
    var webDriverUpdate = require('gulp-protractor').webdriver_update;
    argv['dont-update-wd'] ? next() : webDriverUpdate(next);
}

//
// ******* Common parts *******
//

/**
 * Build index.html
 */
function buildIndexHtml(isVersionForTests) {
    var task = gulp.src([config.app_files.html, config.assets_dir + '/**/*.manifest.json'], {allowEmpty: true})
        .pipe(errorHandler(handleError))
        .pipe(gulpIf(!state.isDevMode, revCollector()))
        .pipe(gulpIf(isVersionForTests, preprocess({context: {IGZ_TEST_E2E: true}}), preprocess()))
        .pipe(gulpIf(!state.isDevMode, minifyHtml({
            removeComments: true,
            collapseWhitespace: true,
            collapseInlineTagWhitespace: true,
            conservativeCollapse: true
        })))
        .pipe(gulp.dest(config.build_dir));

    if (livereload !== null) {
        task.pipe(livereload());
    }

    return task;
}

function buildConfigFromArgs() {
    var buildConfig = {
        mode: argv['demo']    === true ? 'demo'       : // demo overrides staging in case of: `gulp --demo --staging`
              argv['staging'] === true ? 'staging'    :
              /* default */              'production'
    };

    if (state.isDevMode) {
        buildConfig.i18nextExpirationTime = 0;
    }

    // if at least one URL was set, create the config
    // eslint-disable-next-line
    return !lodash.isEmpty(buildConfig) ? JSON.stringify(buildConfig) : null;
}

//
// ******* Task chains *******
//

/**
 * Base build task
 */
function build(next) {
    gulp.series(lint, clean, gulp.parallel(vendorCss, vendorJs), gulp.parallel(appCss, appJs, fonts, images, i18n, monaco), indexHtml, dashboardConfigJson)(next);
}

/**
 * Task for unit test running
 * Task for development environment only
 */
function testUnit(next) {
    gulp.series(setTesting, build, serveStatic, stopServer, testUnitRun)(next);
}

/**
 * Task for e2e test running
 * Task for development environment only
 */
function testE2e(next) {
    gulp.series(e2eHelp, updateWebDriver, setE2eTesting, build, serveStatic, testE2eMockModule, testE2eMockHtml, testE2eRun, stopServer)(next);
}

/**
 * Task for unit and e2e test running (run without tags, using simple state mode)
 * Task for development environment only
 */
function test(next) {
    gulp.series(testUnit, testE2e)(next);
}

/**
 * Lifts up preview server
 * This could be used to quickly use dashboard when it is already built.
 */
function lift(next) {
    var mocks = [serveStatic];

    gulp.parallel(...mocks)(next);
}

/**
 * Default task
 */
function defaultTask(next) {
    gulp.series(gulp.parallel(cleanShared, buildShared), build, lift)(next);
}

/**
 * Build project, watch for changes and build needed sources
 * Task for development environment only
 */
function watch(next) {
    state.isDevMode = true;
    gulp.series(defaultTask, watcher)(next);
}

//
// Shared
//

/**
 * Clean build directory
 */
function cleanShared() {
    if (state.isDevMode) {
        return gulp.src(config.shared_files.dist, {allowEmpty: true})
            .pipe(errorHandler(handleError))
            .pipe(vinylPaths(del));
    }
}

/**
 * Build shared less file (include all shared less files)
 */
function appLessShared() {
    var distFolder = config.shared_files.dist + '/less';

    var appLess = gulp
        .src(config.shared_files.less)
        .pipe(errorHandler(handleError))
        .pipe(concat(config.shared_output_files.app.less))
        .pipe(gulp.dest(distFolder));

    var vendorLess = gulp
        .src(config.shared_files.vendor.less)
        .pipe(errorHandler(handleError))
        .pipe(concat(config.shared_output_files.vendor.less))
        .pipe(gulp.dest(distFolder));

    return merge2(appLess, vendorLess);
}

/**
 * Build app.js (include all project js files and templates)
 */
function appJsShared() {
    var distFolder = config.shared_files.dist + '/js';

    var js = gulp.src(config.shared_files.js, {allowEmpty: true})
        .pipe(errorHandler(handleError))
        .pipe(cache({
            path: config.shared_cache_file,
            transformStreams: [
                babel()
            ]
        }));

    var vendorJsTask = gulp.src(config.shared_files.vendor.js, {allowEmpty: true})
        .pipe(errorHandler(handleError))
        .pipe(concat(config.shared_output_files.vendor.js))
        .pipe(gulp.dest(distFolder));

    var templates = gulp.src(config.shared_files.templates, {allowEmpty: true})
        .pipe(errorHandler(handleError))
        .pipe(minifyHtml({
            removeComments: true,
            collapseWhitespace: true,
            collapseInlineTagWhitespace: true,
            conservativeCollapse: true
        }))
        .pipe(ngHtml2Js({
            moduleName: config.shared_files.templates_module_name
        }));

    var task = merge2(js, templates)
        .pipe(errorHandler(handleError))
        .pipe(concat(config.shared_output_files.app.js))
        .pipe(gulp.dest(distFolder));

    return merge2(task, vendorJsTask);
}

/**
 * Copy all fonts to the build directory
 */
function fontsShared() {
    var distFolder = config.shared_files.dist + '/fonts';

    return gulp.src(config.shared_files.fonts, {allowEmpty: true})
        .pipe(errorHandler(handleError))
        .pipe(gulp.dest(distFolder));
}

/**
 * Copy all translation files to the build directory
 */
function i18nShared() {
    var distFolder = config.shared_files.dist + '/i18n';

    return gulp.src(config.shared_files.i18n, {allowEmpty: true})
        .pipe(errorHandler(handleError))
        .pipe(gulp.dest(distFolder));
}

/**
 * Optimize all images and copy them to the build directory
 */
function imagesShared() {
    var distFolder = config.shared_files.dist + '/images';

    return gulp.src(config.shared_files.images, {allowEmpty: true})
        .pipe(errorHandler(handleError))
        .pipe(imagemin({
            optimizationLevel: 3,
            progressive: true,
            interlaced: true
        }))
        .pipe(gulp.dest(distFolder));
}

/**
 * Lint source code
 */
function lintShared() {
    return gulp.src(config.shared_files.js, {allowEmpty: true})
        .pipe(errorHandler(handleError))
        .pipe(eslint())
        .pipe(eslint.format('compact'))
        .pipe(eslint.failAfterError());
}

function injectVersionShared(next) {
    exec('git describe --tags --abbrev=40', function (err, stdout) {
        buildVersion = stdout;
        next();
    });
}

//
// ******* Task chains *******
//

/**
 * Base build task
 */
function buildShared(next) {
    if (state.isDevMode) {
        gulp.series(lintShared, injectVersionShared, gulp.parallel(appLessShared, appJsShared, fontsShared, imagesShared, i18nShared))(next);
    } else {
        next();
    }
}

//
// Helper methods
//

/**
 * Error handler.
 * @param {Object} error
 */
function handleError(error) {
    console.error(error.message);

    process.exit(1);
}

var Imports = require(__dirname + '/test-utils/imports.repository.js');
global.e2e_root = __dirname;
global.e2e_imports = new Imports();

var config = require(__dirname + '/../../dist/dashboard-config.json');
global.mode = config.mode;
global.configURL = config.url;

global._ = require('lodash');
global.moment = require('moment');

exports.config = {
    specs: 'specs/**/*.spec.js',
    directConnect: true,
    capabilities: {
        browserName: 'chrome',
        chromeOptions: {
            args: [
                '--window-size=1064,800',
                '--disable-extensions',
                '--disable-infobars'
            ],
            prefs: {
                download: {
                    prompt_for_download: false,
                    directory_upgrade: true,
                    default_directory: __dirname + '/downloads'
                }
            }
        },
        loggingPrefs: {
            'browser': 'WARNING'
        }
    },
    params: {
        use_junit_reporter: false
    },
    allScriptsTimeout: 300000,
    jasmineNodeOpts: {
        defaultTimeoutInterval: 300000,
        grep: '#' + mode
    },
    framework: 'jasmine2',
    onPrepare: function () {
        e2e_imports.testUtil.browserUtils().DisableAngularAnimateMockModule().inject();
        e2e_imports.testUtil.browserUtils().DisableCssAnimationMockModule().inject();
    },
    onComplete: function () {
        browser.close();
    }
};
var HtmlReporter = require('protractor-jasmine2-screenshot-reporter');
var jasmineReporters = require('jasmine-reporters');
var fs = require('fs');

module.exports = function () {

    this.createInstance = function (container, specName) {
        var useJUnitReporter = browser.params.use_junit_reporter;
        prepare(container, specName);
        createReporter(container, specName, useJUnitReporter);
    };

    function createReporter(container, specName, useJUnitReporter) {
        var reportPath = e2e_root + '/test-output/' + container + '/';

        console.info('report: ' + container + '/' + specName + '.html');

        if (useJUnitReporter) {
            console.info('report: ' + container + '/' + specName + '.xml');
            jasmine.getEnv().addReporter(new jasmineReporters.JUnitXmlReporter({
                savePath: reportPath,
                filePrefix: specName
            }));
        }

        jasmine.getEnv().addReporter(new HtmlReporter({
            dest: reportPath,
            filename: specName + '.html',
            reportTitle: container + ': ' + specName + '.spec | iguaz.io',
            reportOnlyFailedSpecs: true,
            captureOnlyFailedSpecs: true,
            pathBuilder: function (currentSpec, currentSuite) {
                return specName + '/' + currentSpec.id;
            }
        }));
    }

    function prepare(container, specName) {
        var reportOutputFolder = e2e_root + '/test-output';
        var reportContainerFolder = reportOutputFolder + '/' + container;
        var reportScreenFolder = reportContainerFolder + '/' + specName;
        var reportHtmlFile = reportContainerFolder + '/' + specName + '.html';
        var reportXmlFile = reportContainerFolder + '/' + specName + '.xml';

        // create 'test-output' folder if it needed
        if (!fs.existsSync(reportOutputFolder)) {
            fs.mkdirSync(reportOutputFolder);
        }

        // create container folder if it needed
        if (!fs.existsSync(reportContainerFolder)) {
            fs.mkdirSync(reportContainerFolder);
        }

        // remove existing screens
        if (fs.existsSync(reportScreenFolder)) {
            fs.readdirSync(reportScreenFolder)
                .forEach(function (file, index) {
                    var curPath = reportScreenFolder + "/" + file;

                    // delete file
                    fs.unlinkSync(curPath);
                });
            fs.rmdirSync(reportScreenFolder);
        }

        // remove existing HTML report
        if (fs.existsSync(reportHtmlFile)) {
            fs.unlinkSync(reportHtmlFile);
        }
        // remove existing XML report
        if (fs.existsSync(reportXmlFile)) {
            fs.unlinkSync(reportXmlFile);
        }
    }
};
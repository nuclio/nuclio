function BrowserUtils() {
    var defaultWindowResolution = {width: 1064, height: 800};
    var default_logs_length = 0;

    /**
     * Returns browser's console logs
     * @returns {!webdriver.promise.Promise.<Array>}
     */
    this.getLogs = function () {
        return browser.manage().logs().get('browser').then(function (logs) {
            if (logs.length > default_logs_length) {
                default_logs_length = 0;
                return logs;
                //return [];
            } else {
                return [];
            }
        });
    };

    /**
     * Returns browser's console log message
     * @returns {boolean}
     */
    this.getLogMessage = function () {
        return browser.manage().logs().get('browser').then(function (logs) {
            if (logs.length > default_logs_length) {
                if (logs[0].level.value > 900) { // it's an error log
                    return logs[0].message
                }
            }
        });
    };

    /**
     * Returns browser's resolution
     * @returns {!promise.Promise.<{width: number, height: number}>}
     */
    this.getScreenResolution = function () {
        return browser.driver.manage().window().getSize();
    };

    /**
     * Set logs count of browser's console
     * @param {number} count
     * @returns {!webdriver.promise.Promise.<Array>}
     */
    this.setLogsCount = function (count) {
        default_logs_length = count;
    };

    /**
     *
     * Wait until given function returns true
     * @param {string} description - description of expectation
     * @param {function} condition - function that checks expected condition
     * @param {number | undefined} timeOut - count of milliseconds
     * @returns {!webdriver.promise.Promise}
     */
    this.waitForCondition = function (description, condition, timeOut) {
        timeOut = timeOut || 3000;
        return browser.wait(new protractor.until.Condition(description, condition), timeOut)
            // try-catch block for Promise
            .then(function (success) {     // on success handler
                return success;
            }, function (error) {   // on error handler
                return error;
            });
    };

    /**
     * Returns browser's console logs
     * @returns {!webdriver.promise.Promise.<Array>}
     */
    this.getTitle = function () {
        return browser.getTitle();
    };

    /**
     * Perform page scrolling to given coordinates
     * @param {Object} coordinates - object that contains values x, y in pixels. coordinates from top left page's angle
     * @returns {!webdriver.promise.Promise}
     */
    this.scrollTo = function (coordinates) {
        return browser.waitForAngular()
            .then(function () {
                return browser.executeScript('window.scrollTo(' + coordinates.x + ', ' + coordinates.y + ');');
            })
    };

    /**
     * Run given script on browser
     * @param {string|function} script
     * @returns {!webdriver.promise.Promise}
     */
    this.executeScript = function (script) {
        return browser.waitForAngular()
            .then(function () {
                return browser.executeScript(script);
            })
    };

    /**
     * Resize browser's window to given resolution
     * @param {Object} newResolution - object that contains width and height values in pixels
     * @returns {!webdriver.promise.Promise}
     */
    this.resizeWindowTo = function (newResolution) {
        return browser.waitForAngular()
            .then(function () {
                return browser.driver.manage().window().setSize(newResolution.width, newResolution.height);
            })
    };

    /**
     * Resize browser's window to default resolution
     * @returns {!webdriver.promise.Promise}
     */
    this.resizeWindowToDefault = function () {
        return browser.waitForAngular()
            .then(function () {
                return browser.driver.manage().window().setSize(defaultWindowResolution.width, defaultWindowResolution.height);
            })
    };

    /**
     * MockModule disableAngularAnimate
     * @returns {Object}
     */
    this.DisableAngularAnimateMockModule = function () {

        function injectDisableAngularAnimateMockModule() {
            var disableAngularAnimate = function () {
                angular.module('disableAngularAnimate', []).run(function ($animate) {
                    // disable angular $animate
                    $animate.enabled(false);
                });
            };

            browser.addMockModule('disableAngularAnimate', disableAngularAnimate);
        }

        function removeDisableAngularAnimateMockModule() {
            browser.removeMockModule('disableAngularAnimate');
        }

        return {
            inject: injectDisableAngularAnimateMockModule,
            remove: removeDisableAngularAnimateMockModule
        };
    };

    /**
     * MockModule disableCssAnimation
     * @returns {Object}
     */
    this.DisableCssAnimationMockModule = function () {
        function injectDisableCssAnimationMockModule() {
            // disable animations when testing
            var disableCssAnimation = function () {
                angular.module('disableCssAnimation', []).run(function () {

                    // disable css animations
                    var style = document.createElement('style');
                    style.type = 'text/css';
                    style.innerHTML = '* {' +
                        '-webkit-transition-duration: 1ms !important;' +
                        '-moz-transition-duration: 1ms !important;' +
                        '-o-transition-duration: 1ms !important;' +
                        '-ms-transition-duration: 1ms !important;' +
                        'transition-duration: 1ms !important;' +
                        '-webkit-animation-duration: 1ms !important;' +
                        '-moz-animation-duration: 1ms !important;' +
                        '-o-animation-duration: 1ms !important;' +
                        '-ms-animation-duration: 1ms !important;' +
                        'animation-duration: 1ms !important;' +
                        '}';
                    document.getElementsByTagName('head')[0].appendChild(style);
                });
            };

            browser.addMockModule('disableCssAnimation', disableCssAnimation);
        }

        function removeDisableCssAnimationMockModule() {
            browser.removeMockModule('disableCssAnimation');
        }

        return {
            inject: injectDisableCssAnimationMockModule,
            remove: removeDisableCssAnimationMockModule
        };
    };

    this.sessionClear = function () {
        return browser.manage().deleteAllCookies().then(function () {
            return browser.executeScript("window.localStorage.clear();");
        })
    };

    /**
     * Return period from set date to current
     * @returns {!webdriver.promise.Promise}
     */
    this.getTimeDurationValue = function (createdDate, endDate) {
        var timeAgoValue = '';
        var COUNT_OF_DAYS_IN_WEEK = 7;

        createdDate = moment(createdDate).toISOString();
        endDate = moment(endDate);
        if (!endDate.isValid()) {
            endDate = moment();
        }

        if (moment(createdDate).isBefore(endDate)) {
            var diff = moment(createdDate).diff(endDate);
            var duration = moment.duration(diff);
            var exceptFoundYears = moment(createdDate).add(Math.abs(duration.years()), 'years');
            var weeks = Math.abs(moment(exceptFoundYears).diff(endDate, 'weeks'));
            var formattedDuration = [
                {
                    unit: 'yr',
                    value: Math.abs(duration.years())
                },
                {
                    unit: 'w',
                    value: weeks
                },
                {
                    unit: 'd',
                    value: Math.abs(moment(exceptFoundYears).diff(endDate, 'days')) % COUNT_OF_DAYS_IN_WEEK
                },
                {
                    unit: 'hr',
                    value: Math.abs(duration.hours())
                },
                {
                    unit: 'min',
                    value: Math.abs(duration.minutes())
                },
                {
                    unit: 'sec',
                    value: Math.abs(duration.seconds())
                }
            ];

            timeAgoValue = _.chain(formattedDuration)
                .dropWhile(function (item) {
                    return item.value === 0;
                })
                .map(function (item) {
                    return item.value + ' ' + item.unit;
                })
                .value();

            timeAgoValue = timeAgoValue.shift() +
                (_.isEmpty(timeAgoValue) || _(timeAgoValue).head() === '0' ? '' : ' ' + timeAgoValue.shift());
        }

        return timeAgoValue;
    };

    /**
     * Returns on previous page using browser's "Back" button
     * @returns {!webdriver.promise.Promise}
     */
    this.navigateBack = function () {
        return browser.waitForAngular().then(function () {
            return browser.navigate().back();
        });
    };
}

module.exports = BrowserUtils;
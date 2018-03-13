module.exports = function () {
    var WAIT_TIMEOUT = 4000;
    var WAIT_FOR_ELEMENTS_ARRAY_TIMEOUT = 3000;
    var conditions = protractor.ExpectedConditions;

    /**
     * Returns an instance of Control object that contains ElementFinder element by given locator.
     * @param {webdriver.Locator} locator
     * @returns {Control}
     */
    this.get = function (locator) {
        return new Control(element(locator));
    };

    /**
     * Returns an instance of ControlsArray object that contains ElementArrayFinder element by given locator.
     * @param {webdriver.Locator} locator
     * @returns {ControlsArray}
     */
    this.all = function (locator) {
        return new ControlsArray(element.all(locator));
    };

    /**
     * An object that contain methods for work with elements in browser. Actually is wrapper for ElementFinder element.
     * @param {ElementFinder} element_finder
     * @constructor
     */
    function Control(element_finder) {

        /**
         * Clicks on this element.
         * @returns {!webdriver.promise.Promise.<void>}
         */
        this.click = function () {
            return waitForVisibility()
                .then(function () {
                    return browser.wait(conditions.elementToBeClickable(element_finder), WAIT_TIMEOUT);
                })
                .then(function () {
                    return browser.actions().mouseMove(element_finder, undefined)
                        .perform()
                        .then(function () {
                            return element_finder.click();
                        })
                }
                , function (e) {
                    throw 'Wait timed out after ' + WAIT_TIMEOUT + ' of waiting for element to be clickable:\n' + element_finder.locator().toString();
                });
        };

        /**
         * Returns the visible (i.e. not hidden by CSS) innerText of this element, including sub-elements, without any leading or trailing whitespace.
         * @returns {!webdriver.promise.Promise.<string>}
         */
        this.getText = function () {
            return waitForVisibility()
                .then(function () {
                    return element_finder.getText();
                });
        };

        /**
         * Type the given characters sequence to visible element.
         * @param {string | webdriver.Key} text
         * @returns {!webdriver.promise.Promise.<void>}
         */
        this.sendKeys = function (text) {
            return waitForVisibility().then(function () {
                return element_finder.sendKeys(protractor.Key.END, text);
            });
        };

        /**
         * Type the given characters sequence to present element.
         * @param {string} text
         * @returns {!webdriver.promise.Promise.<void>}
         */
        this.sendKeysToHiddenElement = function (text) {
            return waitForPresence()
                .then(function () {
                    return element_finder.sendKeys(text);
                });
        };

        /**
         * Clears the value of this element.
         * @returns {!webdriver.promise.Promise.<void>}
         */
        this.clear = function () {
            return waitForPresence()
                .then(function () {
                    return element_finder.clear();
                });
        };

        /**
         * Returns value of the given attribute of this element.
         * @param {string} attribute
         * @returns {!webdriver.promise.Promise.<string>}
         */
        this.getAttribute = function (attribute) {
            return waitForPresence()
                .then(function () {
                    return element_finder.getAttribute(attribute)
                });
        };

        /**
         * Checks whether the element contains contains given value of attribute.
         * @param {string} attribute
         * @param {string} value
         * @returns {!webdriver.promise.Promise.<boolean>}
         */
        this.isAttributeContainingValue = function (attribute, value, contains) {
            return waitForPresence()
                .then(function () {
                    return element_finder.getAttribute(attribute)
                        .then(function (attributeValue) {
                            if (contains === false) {
                                return (attributeValue ? (attributeValue.indexOf(value) < 0) : true);
                            } else {
                                return (attributeValue.indexOf(value) > -1);
                            }
                        });
                });
        };

        /**
         * Checks whether the element is enabled
         * @returns {!webdriver.promise.Promise.<boolean>}
         */
        this.isEnabled = function () {
            return element_finder.isEnabled();
        };

        /**
         * Checks whether the element visibility match given expected visibility value.
         * @param {boolean} isVisible
         * @returns {!webdriver.promise.Promise.<boolean>}
         */
        this.isVisibility = function (isVisible) {
            return waitForVisibilityValue(isVisible);
        };

        /**
         * Checks whether the element presence match given expected presence value.
         * @param {boolean} isPresent
         * @returns {!webdriver.promise.Promise.<boolean>}
         */
        this.isPresence = function (isPresent) {
            return waitForPresenceValue(isPresent);
        };

        /**
         * Checks whether the element present in DOM.
         * @returns {ElementFinder}
         */
        this.isPresent = function () {
            return element_finder.isPresent();
        };

        /**
         * Checks whether the element is selected. Should be used for dropdown options, check boxes and radio buttons.
         * @returns {!webdriver.promise.Promise.<boolean>}
         */
        this.isSelected = function () {
            return waitForPresence()
                .then(function () {
                    return element_finder.isSelected();
                })
        };

        /**
         * Returns a size of this element's bounding box, in pixels.
         * @returns {!webdriver.promise.Promise.<{width: number, height: number}>}
         */
        this.getSize = function () {
            return waitForPresence()
                .then(function () {
                    return element_finder.getSize();
                });
        };

        /**
         *
         * Returns a location of this element in pixels.
         * @returns {!webdriver.promise.Promise.<{x: number, y: number}>}
         */
        this.getLocation = function () {
            return waitForPresence()
                .then(function () {
                    return element_finder.getLocation();
                });
        };

        /**
         * Scroll browser window to this element
         * @returns {!webdriver.promise.Promise}
         */
        this.scrollScreenToElement = function () {
            return browser.manage().window().getSize()
                .then(function (resolution) {
                    return element_finder.getLocation()
                        .then(function (coordinates) {
                            return browser.executeScript('window.scrollTo(' + (coordinates.x - resolution.width / 2) + ', ' + (coordinates.y - resolution.height / 2) + ');');
                            //return browser.executeScript("$('.igz-info-page-content .mCSB_container')[0].style.top = '-" + (coordinates.y - resolution.height / 2) + "px'");
                        })
                });
        };

        /**
         * Returns an ElementFinder instance assigned to this Control object.
         * @returns {ElementFinder}
         */
        this.getElementFinder = function () {
            return element_finder;
        };

        /**
         * Returns a string value of locator content.
         * @returns {string}
         */
        this.getLocator = function () {
            return element_finder.locator().value;
        };

        /**
         * Wait until element's visibility value not equal for expected.
         * @param {boolean} isVisible
         * @returns {!webdriver.promise.Promise.<boolean>}
         */
        function waitForVisibilityValue(isVisible) {
            if (isVisible) {
                return waitForVisibility()
                    .then(function (visibility) {
                        return visibility;
                    },
                    function (e) {
                        return false;
                    });
            } else {
                return waitForInvisibility()
                    .then(function (invisibility) {
                        return invisibility;
                    },
                    function (e) {
                        return false;
                    });
            }
        }

        /**
         * Wait until element's presence value not equal for expected.
         * @param {boolean} isPresent
         * @returns {!webdriver.promise.Promise.<boolean>}
         */
        function waitForPresenceValue(isPresent) {
            if (isPresent) {
                return waitForPresence()
                    .then(function (presence) {
                        return presence
                    },
                    function (e) {
                        return false;
                    });
            } else {
                return waitForStaleness()
                    .then(function (presence) {
                        return presence
                    },
                    function (e) {
                        return false;
                    });
            }
        }

        /**
         * Checks whether the element is present on the DOM of a page and visible.
         * @returns {!webdriver.promise.Promise.<boolean>}
         */
        function waitForVisibility() {
            return waitForPresence()
                .then(function () {
                    return browser.wait(conditions.visibilityOf(element_finder), WAIT_TIMEOUT);
                })
                .then(function (visibility) {
                    return visibility
                },
                function () {
                    throw 'Wait timed out after ' + WAIT_TIMEOUT + ' of waiting for element visibility:\n' + element_finder.locator().toString();
                });
        }

        /**
         * Checks whether the element is either invisible or not present on the DOM.
         * @returns {!webdriver.promise.Promise.<boolean>}
         */
        function waitForInvisibility() {
            return browser.waitForAngular()
                .then(function () {
                    return browser.wait(conditions.invisibilityOf(element_finder), WAIT_TIMEOUT);
                })
                .then(function (invisibility) {
                    return invisibility
                },
                function () {
                    throw 'Wait timed out after ' + WAIT_TIMEOUT + ' of waiting for element invisibility:\n' + element_finder.locator().toString();
                });
        }

        /**
         * Checks whether the element is present on the DOM of a page.
         * @returns {!webdriver.promise.Promise.<boolean>}
         */
        function waitForPresence() {
            return browser.waitForAngular()
                .then(function () {
                    return browser.wait(conditions.presenceOf(element_finder), WAIT_TIMEOUT);
                })
                .then(function (presence) {
                    return presence
                },
                function () {
                    throw 'Wait timed out after ' + WAIT_TIMEOUT + ' of waiting for element presence:\n' + element_finder.locator().toString();
                });
        }

        /**
         * Checks whether the element is not attached to the DOM of a page.
         * @returns {!webdriver.promise.Promise.<boolean>}
         */
        function waitForStaleness() {
            return browser.waitForAngular()
                .then(function () {
                    return browser.wait(conditions.stalenessOf(element_finder), WAIT_TIMEOUT);
                })
                .then(function (staleness) {
                    return staleness
                },
                function () {
                    throw 'Wait timed out after ' + WAIT_TIMEOUT + ' of waiting for element staleness:\n' + element_finder.locator().toString();
                });
        }
    }

    /**
     * An object that contain methods for work with elements in browser. Actually is wrapper for ElementArrayFinder element.
     * @param {ElementArrayFinder} element_array_finder
     * @constructor
     */
    function ControlsArray(element_array_finder) {

        /**
         * Returns an array of visibile text values for all elements in element array finder.
         * @returns {!webdriver.promise.Promise.<Array>}
         */
        this.getVisibleTextList = function () {
            var textList = [];

            return waitForListElements()
                .then(function () {
                    return element_array_finder
                        .each(function (element, index) {
                            return element.getText()
                                .then(function (text) {
                                    var result = text.trim();
                                    if (result.length > 0) {
                                        textList.push(result);
                                    }
                                });
                        })
                        .then(function () {
                            return textList;
                        });
                });
        };

        /**
         * Returns an array of text values for all elements in element array finder.
         * @returns {!webdriver.promise.Promise.<Array>}
         */
        this.getTextList = function () {
            var textList = [];

            return waitForListElements()
                .then(function () {
                    return element_array_finder
                        .each(function (element, index) {
                            return element.getText()
                                .then(function (text) {
                                    textList[index] = text.trim();
                                });
                        })
                })
                .then(function () {
                    return textList;
                });
        };

        /**
         * Returns an array of given attribute values for all elements in element array finder.
         * @param {string} attribute
         * @returns {!webdriver.promise.Promise.<Array>}
         */
        this.getAttributesList = function (attribute) {
            var valuesList = [];

            return waitForListElements()
                .then(function () {
                    return element_array_finder
                        .each(function (element, index) {
                            return element.getAttribute(attribute)
                                .then(function (attributeValue) {
                                    if (typeof(attributeValue) != typeof('')) {
                                        attributeValue = '';
                                    }
                                    valuesList[index] = attributeValue.trim();
                                });
                        })
                })
                .then(function () {
                    return valuesList;
                });
        };

        /**
         * Checks whether the all elements in element array finder contain given attribute values.
         * @param {string} attribute
         * @param {string} value
         * @param {boolean} expectedResult
         * @returns {!webdriver.promise.Promise.<boolean>}
         */
        this.areAttributesContainingValue = function (attribute, value, expectedResult) {
            if (typeof expectedResult === 'undefined') {
                expectedResult = true;
            }
            var isContaining = expectedResult;

            return waitForListElements()
                .then(function () {
                    return element_array_finder
                        .each(function (element, index) {
                            return element.getAttribute(attribute)
                                .then(function (attributeValue) {
                                    if (typeof(attributeValue) != typeof('')) {
                                        attributeValue = '';
                                    }
                                    if (expectedResult) {
                                        isContaining = isContaining && (attributeValue.indexOf(value) > -1);
                                    } else {
                                        isContaining = isContaining || (attributeValue.indexOf(value) > -1);
                                    }
                                });
                        })
                })
                .then(function () {
                    return isContaining === expectedResult;
                });
        };

        this.areAttributesContainingValues = function (attribute, values, expectedResult) {
            if (typeof expectedResult === 'undefined') {
                expectedResult = true;
            }
            var isContaining = expectedResult;

            return waitForListElements()
                .then(function () {
                    return element_array_finder
                        .each(function (element, index) {
                            return element.getAttribute(attribute)
                                .then(function (attributeValue) {
                                    if (typeof(attributeValue) != typeof('')) {
                                        attributeValue = '';
                                    }
                                    if (expectedResult) {
                                        isContaining = isContaining && (attributeValue.indexOf(values[index]) > -1);
                                    } else {
                                        isContaining = isContaining || (attributeValue.indexOf(values[index]) > -1);
                                    }
                                });
                        })
                })
                .then(function () {
                    return isContaining === expectedResult;
                });
        };

        /**
         * Returns an array of matches the given attribute values in array finder elements.
         * @param {string} attribute
         * @param {string} value
         * @returns {!webdriver.promise.Promise.<boolean>}
         */
        this.getAttributesValuesMatchesList = function (attribute, value) {
            var valuesList = [];

            return waitForListElements()
                .then(function () {
                    return element_array_finder
                        .each(function (element) {
                            return element.getAttribute(attribute)
                                .then(function (attributeValue) {
                                    console.log(attributeValue);
                                    valuesList.push(attributeValue.indexOf(value) > -1);
                                });
                        })
                })
                .then(function () {
                    return valuesList;
                });
        };

        /**
         * Returns a string value of locator content.
         * @returns {string}
         */
        this.getLocator = function () {
            return element_array_finder.locator().value;
        };

        /**
         * Returns a count elements in element array finder
         * @returns {!webdriver.promise.Promise.<number>}
         */
        this.getCount = function () {
            return waitForListElements()
                .then(function () {
                    return element_array_finder.count();
                });
        };

        /**
         * Returns a first elements in element array finder
         * @returns {Control}
         */
        this.getFirst = function () {
            waitForListElements();
            return new Control(element_array_finder.first())
        };

        /**
         * Returns a last elements in element array finder
         * @returns {Control}
         */
        this.getLast = function () {
            waitForListElements();
            return new Control(element_array_finder.last())
        };

        /**
         * Returns a elements by index in element array finder
         * @returns {Control}
         */
        this.getByIndex = function (index) {
            waitForListElements();
            return new Control(element_array_finder.get(index))
        };

        /**
         * Wait for element_array_finder not empty and catch timeout exception
         * @returns {!webdriver.promise.Promise}
         */
        function waitForListElements() {
            var error_text = 'elements array not empty';
            return browser.waitForAngular()
                .then(function () {
                    return browser.wait(new protractor.until.Condition(error_text, function () {
                        return element_array_finder.count().then(function (count) {
                            return count > 0
                        });
                    }), WAIT_FOR_ELEMENTS_ARRAY_TIMEOUT)
                }).then(function () {}, function (error) {});
        }
    }
};

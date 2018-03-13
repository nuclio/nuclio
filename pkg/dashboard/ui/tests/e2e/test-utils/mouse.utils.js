module.exports = function () {
    /**
     * Left click on element and his coordinates
     * @param {Control} element - web element that will be clicked
     * @param {Object} coordinates - object that contains values x, y in pixels. coordinates from top left element's angle
     * @returns {!webdriver.promise.Promise}
     */
    this.click = function (element, coordinates) {
        return getVisible(element)
            .then(function (visibleElement) {
                return browser.actions()
                    .mouseMove(visibleElement, coordinates)
                    .click()
                    .perform();
            });
    };

    /**
     * Right click on element and his coordinates
     * @param {Control} element - web element that will be clicked
     * @param {Object} coordinates - object that contains values x, y in pixels. coordinates from top left element's angle
     * @returns {!webdriver.promise.Promise}
     */
    this.rightClick = function (element, coordinates) {
        return getVisible(element)
            .then(function (visibleElement) {
                return browser.actions()
                    .mouseMove(visibleElement, coordinates)
                    .click(protractor.Button.RIGHT)
                    .perform();
            });
    };

    /**
     * Double click on element and his coordinates
     * @param {Control} element - web element that will be clicked
     * @param {Object} coordinates - object that contains values x, y in pixels. coordinates from top left element's angle
     * @returns {!webdriver.promise.Promise}
     */
    this.doubleClick = function (element, coordinates) {
        return getVisible(element)
            .then(function (visibleElement) {
                return browser.actions()
                    .mouseMove(visibleElement, coordinates)
                    .doubleClick()
                    .perform();
            });
    };

    /**
     * Press right mouse button on element and drag it
     * @param {Control} startElement - start point web element that will be clicked
     * @param {Object} startpoint - object that contains values x, y in pixels - coordinates from top left start element's angle
     * @param {Control} endElement - end point web element
     * @param {Object} endpoind - object that contains values x, y in pixels - coordinates from top left end element's angle
     * @returns {!webdriver.promise.Promise}
     */
    this.drag = function (startElement, startpoint, endElement, endpoind) {
        return getVisible(startElement)
            .then(function (visibleStartElement) {
                return browser.actions()
                    .mouseMove(visibleStartElement, startpoint)
                    .mouseDown()
                    .perform();
            })
            .then(function () {
                    return getVisible(endElement)
            })
            .then(function (visibleEndElement) {
                return browser.actions()
                    .mouseMove(visibleEndElement, endpoind)
                    .perform();
            });
    };

    /**
     * Unpress right mouse button after dragging
     * @returns {!webdriver.promise.Promise}
     */
    this.drop = function () {
        return browser.actions()
            .mouseUp()
            .perform();
    };

    /**
     * Press right mouse button on element drag and drop it
     * @param {Control} startElement - start point web element that will be clicked
     * @param {Object} startpoint - object that contains values x, y in pixels - coordinates from top left start element's angle
     * @param {Control} endElement - end point web element
     * @param {Object} endpoind - object that contains values x, y in pixels - coordinates from top left end element's angle
     * @returns {!webdriver.promise.Promise}
     */
    this.dragAndDrop = function (startElement, startpoint, endElement, endpoind) {
        return getVisible(startElement)
            .then(function (visibleStartElement) {
                return browser.actions()
                    .mouseMove(visibleStartElement, startpoint)
                    .mouseDown()
                    .perform()
            })
            .then(function () {
                return getVisible(endElement)
            })
            .then(function (visibleEndElement) {
                return browser.actions()
                    .mouseMove(visibleEndElement, endpoind)
                    .mouseUp()
                    .perform();
            });
    };

    /**
     * Move mouse to given element
     * @param {Control} element
     * @returns {!webdriver.promise.Promise}
     */
    this.move = function (element) {
        return getVisible(element)
            .then(function (visibleStartElement) {
                return browser.actions()
                    .mouseMove(visibleStartElement)
                    .perform()
            });
    };

    /**
     * Move mouse in Y coordinate axis
     * @param {Control} startElement - start point web element
     * @param {Object} pixels - move up on given pixels count.
     * @returns {!webdriver.promise.Promise}
     */
    this.moveY = function (startElement, pixels) {
        return getVisible(startElement)
            .then(function (visibleStartElement) {
                return browser.actions()
                    .mouseMove(visibleStartElement, {x: 1, y: -pixels})
                    .perform()
            });
    };

    /**
     * Move mouse in X coordinate axis
     * @param {Control} startElement - start point web element
     * @param {number} pixels - move right on given pixels count.
     * @returns {!webdriver.promise.Promise}
     */
    this.moveX = function (startElement, pixels) {
        return getVisible(startElement)
            .then(function (visibleStartElement) {
                return browser.actions()
                    .mouseMove(visibleStartElement, {x: pixels, y: 1})
                    .perform()
            });
    };

    /**
     * Move mouse in X and Y coordinate axis
     * @param {Control} startElement - start point web element
     * @param {number} xPixels - move right on given pixels count.
     * @param {number} yPixels - move down on given pixels count.
     * @returns {!webdriver.promise.Promise}
     */

    this.moveXY = function (startElement, xPixels, yPixels) {
        return getVisible(startElement)
            .then(function (visibleStartElement) {
                return browser.actions()
                    .mouseMove(visibleStartElement, {x: xPixels, y: yPixels})
                    .perform()
            });
    };

    /**
     * Returns visible control's elementFinder element
     * @param {Control} element
     * @returns {!webdriver.promise.Promise.<ElementFinder>}
     */
    function getVisible(element) {
        return element.isVisibility(true)
            .then(function () {
                return element.getElementFinder()
            });
    }
};
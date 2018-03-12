module.exports = function () {

    this.pressButtons = function (keyboardButton) {
        return browser.actions().sendKeys(keyboardButton).perform();
    };

    /**
     * Press given Button
     * @param {webdriver.Key} keyboardButton
     */
    this.keyDown = function (keyboardButton) {
        return browser.actions().keyDown(keyboardButton).perform();
    };

    /**
     * Press out given Button
     * @param {webdriver.Key} keyboardButton
     */
    this.keyUp = function (keyboardButton) {
        return browser.actions().keyUp(keyboardButton).perform();
    };

    /**
     * Returns Ctrl or Cmd Button
     * @returns {webdriver.Key}
     */
    this.getControlButton = function () {
        return protractor.Key.META || protractor.Key.CONTROL;
    };

    /**
     * Returns Enter Button
     * @returns {webdriver.Key}
     */
    this.getEnterButton = function () {
        return protractor.Key.ENTER;
    };

    /**
     * Returns Enter Button
     * @returns {webdriver.Key}
     */
    this.getESCButton = function () {
        return protractor.Key.ESCAPE;
    };

    /**
     * Returns Arrow Right Button
     * @returns {webdriver.Key}
     */
    this.getArrowRightButton = function () {
        return protractor.Key.ARROW_RIGHT;
    };

    /**
     * Returns Arrow Right Button
     * @returns {webdriver.Key}
     */
    this.getArrowDownButton = function () {
        return protractor.Key.ARROW_DOWN;
    };

    /**
     * Return backspace button
     * @returns {webdriver.Key}
     */
    this.getBackspaceButton = function () {
        return protractor.Key.BACK_SPACE;
    };
};

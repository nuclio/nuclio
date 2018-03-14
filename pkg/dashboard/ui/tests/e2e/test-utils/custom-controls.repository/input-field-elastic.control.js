function InputFieldElastic(parentElementXPath, index) {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var keyboardUtils = e2e_imports.testUtil.keyboardUtils();
    var containerXPath;
    var _this = this;

    index = index || 1;
    containerXPath = "(" + parentElementXPath + "//igz-elastic-input-field)[" + index + "]/div";

    this.container = control.get(by.xpath(containerXPath));
    this.inputField = control.get(by.xpath(containerXPath + "//input[contains(@class, 'elastic-input')]"));

    // methods

    /**
     * Type given text to this input field
     * @param {string} text
     * @returns {!webdriver.promise.Promise}
     */
    this.sendKeys = function (text) {
        return _this.inputField.click().then(function () {
            return _this.inputField.clear()
        }).then(function () {
            return _this.inputField.sendKeys(text)
        });
    };

    /**
     * Check whether the validator border invalid
     * @param {boolean} isInvalid
     * @returns {!webdriver.promise.Promise.<boolean>}
     */
    this.isValidatorBorderInvalid = function (isInvalid) {
        return _this.inputField.isAttributeContainingValue('class', 'ng-invalid').then(function (invalid) {
            return isInvalid === invalid;
        });
    };

    /**
     * Type backspace given count times
     * @param {number} count
     * @returns {!webdriver.promise.Promise}
     */
    this.typeBackspace = function (count) {
        return _.last(_.times(count, function () {
            return _this.inputField.sendKeys(keyboardUtils.getBackspaceButton());
        }));
    };
}

module.exports = InputFieldElastic;
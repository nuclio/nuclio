function InputFieldValidating(parentElementXPath, index) {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var keyboardUtils = e2e_imports.testUtil.keyboardUtils();
    var containerXPath;
    var _this = this;

    index = index || 1;
    containerXPath = "(" + parentElementXPath + "//igz-validating-input-field)[" + index + "]";

    this.container = control.get(by.xpath(containerXPath));
    this.characterCounter = control.get(by.xpath(containerXPath + "//div[contains(@class, '-counter')][not(*)]"));
    this.inputPlaceholder = control.get(by.xpath("(" + parentElementXPath + "//div[contains(@class, '-placeholder')])[1]"));
    this.inputField = control.get(by.xpath(containerXPath + "//div/*[contains(@class, '-field')]"));
    this.errorText = control.get(by.xpath(containerXPath + "/../div[contains(@class,'error')][not(contains(@class,'hide'))]"));

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
        return _this.inputField.isAttributeContainingValue('class', 'invalid').then(function (invalid) {
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

module.exports = InputFieldValidating;
function InputFieldNumber(parentElementXPath, index) {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var keyboardUtils = e2e_imports.testUtil.keyboardUtils();
    var mouseUtils = e2e_imports.testUtil.mouseUtils();
    var containerXPath;
    var _this = this;

    index = index || 1;
    containerXPath = "(" + parentElementXPath + "//igz-number-input)[" + index + "]";

    this.container = control.get(by.xpath(containerXPath));
    this.inputField = control.get(by.xpath(containerXPath + "//*[contains(@class, 'input-field')]"));
    this.prefixLabel = control.get(by.xpath(containerXPath + "//*[contains(@class, 'prefix-unit')]"));
    this.suffixLabel = control.get(by.xpath(containerXPath + "//*[contains(@class, 'suffix-unit')]"));
    this.increaseArrow = control.get(by.xpath(containerXPath + "//*[contains(@class, 'igz-icon-dropup')]"));
    this.decreaseArrow = control.get(by.xpath(containerXPath + "//*[contains(@class, 'igz-icon-dropdown')]"));
    this.validatorLine = control.get(by.xpath(containerXPath + "//*[contains(@class, 'input-status-line')]"));

    // methods
    /**
     * Type given text to this input field
     * @param {string} text
     * @returns {!webdriver.promise.Promise}
     */
    this.sendKeys = function (text) {
        return mouseUtils.click(_this.inputField, {x: 5, y: 5}).then(function () {
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
        return mouseUtils.click(_this.inputField, {x: 5, y: 5}).then(function () {
            return _.last(_.times(count, function () {
                return _this.inputField.sendKeys(keyboardUtils.getBackspaceButton());
            }));
        })
    };
}

module.exports = InputFieldNumber;
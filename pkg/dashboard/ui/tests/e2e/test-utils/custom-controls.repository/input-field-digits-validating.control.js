function InputFieldDigitsValidating(parentElementXPath, index) {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var inputFieldElement;
    var _this = this;

    index = index || 1;
    inputFieldElement = "(" + parentElementXPath + "//*[@data-igz-input-only-digits])[" + index + "]";

    this.inputField = control.get(by.xpath(inputFieldElement));

    // methods
    /**
     * Check whether the validator line invalid
     * @param {boolean} isInvalid
     * @returns {!webdriver.promise.Promise.<boolean>}
     */
    this.isValidatorLineInvalid = function (isInvalid) {
        return _this.inputField.isAttributeContainingValue('class', 'input-invalid').then(function (invalid) {
            return isInvalid === invalid;
        });
    }
}

module.exports = InputFieldDigitsValidating;
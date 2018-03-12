function ChipsInput(parentElementXPath, index) {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var keyboardUtils = e2e_imports.testUtil.keyboardUtils();
    var containerXPath;
    var _this = this;

    index = index || 1;
    containerXPath = "(" + parentElementXPath + "//igz-chips-input)[" + index + "]";

    // controls
    this.container = control.get(by.xpath(containerXPath));
    this.inputField = control.get(by.xpath(containerXPath + "//tags-input//input"));
    this.chipsList = control.all(by.xpath(containerXPath + "//div[contains(@class,'tags')]//span[contains(@class, 'tag-content')]"));
    this.chipXButtonByIndex = function (tagIndex) {
        return control.get(by.xpath("(" + containerXPath + "//tags-input//a[contains(@class,'remove-button')])[" + tagIndex + "]"));
    };
    this.suggestedChipsContainer = control.get(by.xpath(containerXPath + "//div[contains(@class,'autocomplete')]"));
    this.suggestedChipsList = control.all(by.xpath(containerXPath + "//ul[contains(@class,'suggestion-list')]//*[contains(@class, 'tag-content')]"));
    this.suggestedChipByIndex = function (tagIndex) {
        return control.get(by.xpath("(" + containerXPath + "//ul[contains(@class,'suggestion-list')]//*[contains(@class, 'tag-content')])[" + tagIndex + "]"));
    };
    this.suggestedChipsCounter = control.get(by.xpath(containerXPath + "//span[@class='suggestions-counter']"));

    /**
     * Check whether suggested chips containeris visible
     * @param {boolean} isVisible
     * @returns {!webdriver.promise.Promise.<boolean>}
     */
    this.isSuggestionContainerVisible = function (isVisible) {
        return _this.suggestedChipsContainer.isPresence(isVisible)
            .then(function (presence) {
                return isVisible ? _this.suggestedChipsContainer.isVisibility(isVisible) : presence;
            });
    };

    // methods
    /**
     * Type given text to chips input field
     * @returns {!webdriver.promise.Promise}
     */
    this.typeToChipsInput = function (text) {
        return _this.inputField.click().then(function () {
            return _this.inputField.sendKeys(text);
        });
    };

    /**
     * Type backspace twice to chips input field
     * @returns {!webdriver.promise.Promise}
     */
    this.typeBackspaceTwice = function () {
        return _this.inputField.sendKeys(keyboardUtils.getBackspaceButton()).then(function () {
            return _this.inputField.sendKeys(keyboardUtils.getBackspaceButton());
        });
    };

    /**
     * Check whether the validator line invalid
     * @param {boolean} isInvalid
     * @returns {!webdriver.promise.Promise.<boolean>}
     */
    this.isInputFieldInvalid = function (isInvalid) {
        return control.get(by.xpath(containerXPath + "//tags-input")).isAttributeContainingValue('class', 'invalid').then(function (invalid) {
            return invalid === isInvalid;
        });
    };
}
module.exports = ChipsInput;
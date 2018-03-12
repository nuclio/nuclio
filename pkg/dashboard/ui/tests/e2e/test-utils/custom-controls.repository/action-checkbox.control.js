function ActionCheckbox(parentElementXPath, index) {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var containerXPath = parentElementXPath + "//igz-action-checkbox";
    var _this = this;

    index = index || 1;

    this.all = control.all(by.xpath(containerXPath + "/div/div"));
    this.checkbox = control.get(by.xpath("(" + containerXPath + "/div/div)[" + index + "]"));

    /**
     * Check checkbox to given state
     * @param {boolean} check
     * @returns {!webdriver.promise.Promise}
     */
    this.check = function (check) {
        return _this.isCheckboxSelected().then(function (checked) {
            return _this.checkbox.click().then(function () {
                if (check === checked) {
                    return _this.checkbox.click();
                }
            });
        });
    };

    /**
     * Check whether the checkbox checked value match the given value
     * @returns {!webdriver.promise.Promise.<boolean>}
     */
    this.isCheckboxSelected = function () {
        return _this.checkbox.isAttributeContainingValue('class', 'checkbox-checked');
    };

    /**
     * Check whether the all checkboxes checked value match the given value
     * @param {boolean} isChecked
     * @returns {!webdriver.promise.Promise.<boolean>}
     */
    this.areAllThisCheckboxesChecked = function (isChecked) {
        return _this.all.areAttributesContainingValue('class', 'checkbox-checked', isChecked)
    };

}
module.exports = ActionCheckbox;
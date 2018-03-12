function Checkbox(parentElementXPath, index) {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var _this = this;

    index = index || 1;

    this.all = control.all(by.xpath(parentElementXPath + "//input[@type='checkbox']"));
    this.labels = control.all(by.xpath(parentElementXPath + "//input[@type='checkbox']/../label"));
    this.input = control.get(by.xpath("(" + parentElementXPath + "//input[@type='checkbox'])[" + index + "]"));
    this.label = control.get(by.xpath("(" + parentElementXPath + "//input[@type='checkbox']/../label)[" + index + "]"));

    /**
     * Check checkbox to given state
     * @param {boolean} check
     * @returns {!webdriver.promise.Promise}
     */
    this.check = function (check) {
        return _this.input.isSelected().then(function (checked) {
            return _this.label.click().then(function () {
                if (check === checked) {
                    return _this.label.click();
                }
            });
        });
    };

    /**
     * Check whether the all checkboxes checked value match the given value
     * @param {boolean} isChecked
     * @returns {!webdriver.promise.Promise.<boolean>}
     */
    this.areAllThisCheckboxesChecked = function (isChecked) {
        return _this.all.areAttributesContainingValue('checked', 'true', isChecked)
    };

    /**
     * Check whether the all checkboxes checked value match the given value
     * @returns {!webdriver.promise.Promise.<string>}
     */
    this.getAllThisCheckboxesLabels = function () {
        return _this.labels.getAttributesList('textContent');
    };
}
module.exports = Checkbox;
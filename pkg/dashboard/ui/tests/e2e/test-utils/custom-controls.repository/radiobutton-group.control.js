function RadiobuttonGroup(parentElementXPath, index) {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var _this = this;
    index = index || 1;

    this.all = control.all(by.xpath(parentElementXPath + "//input[@type='radio']"));
    this.labels = control.all(by.xpath(parentElementXPath + "//input[@type='radio']/../label"));
    this.input = function (index) {
        return control.get(by.xpath("(" + parentElementXPath + "//input[@type='radio'])[" + index + "]"));
    };
    this.label = function (index) {
        return control.get(by.xpath("(" + parentElementXPath + "//input[@type='radio']/../label)[" + index + "]"));
    };

    /**
     * Check radiobutton to given state
     * @returns {!webdriver.promise.Promise}
     */
    this.check = function (index) {
        return _this.label(index).click();
    };

    /**
     * Check whether the all radiobuttons checked value match the given value
     * @returns {!webdriver.promise.Promise.<string>}
     */
    this.getLabels = function () {
        return _this.labels.getAttributesList('textContent');
    };

    /**
     * Returns a label of selected radiobutton
     * @returns {!webdriver.promise.Promise.<boolean>}
     */
    this.getCheckedRadiobuttonLabel = function () {
        return _this.all.getAttributesValuesMatchesList('checked', 'true').then(function (checkedList) {
            for (var radiobutton in checkedList) {
                if (checkedList[radiobutton]) {
                    return radiobutton
                }
            }
            return null;
        }).then(function (checkedRadiobuttonIndex) {
            return checkedRadiobuttonIndex === null ? '' : control.get(by.xpath("(" + parentElementXPath +
                "//input[@type='radio']/../label)[" + checkedRadiobuttonIndex + "]")).getAttribute('textContent');
        });
    };

    /**
     * Return Radiobutton by index
     * @param {number} index
     * @returns {Radiobutton}
     */
    this.getRadiobuttonByIndex = function (index) {
        return new Radiobutton(index);
    };

    function Radiobutton(index) {
        if (typeof index === 'undefined') {
            index = 1;
        }

        this.input = control.get(by.xpath("(" + parentElementXPath + "//input[@type='radio'])[" + index + "]"));
        this.label = control.get(by.xpath("(" + parentElementXPath + "//input[@type='radio']/../label)[" + index + "]"));
    }
}
module.exports = RadiobuttonGroup;
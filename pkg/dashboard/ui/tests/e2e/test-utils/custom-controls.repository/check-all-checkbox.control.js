function CheckAllCheckbox(index) {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var _this = this;
    index = index || 1;
    var containerXPath = "(//igz-action-checkbox-all//div[contains(@class,'check-item')])[" + index + "]";
    var checkedAllClass = 'igz-icon-checkbox-checked';
    var checkedPartlyClass = 'igz-icon-checkbox-checked-few';

    this.checkbox = control.get(by.xpath(containerXPath));

    /**
     * Returns state of CheckAll checkbox
     * @returns {!webdriver.promise.Promise.<string>}
     */
    this.getCheckboxState = function () {
        return _this.checkbox.getAttribute('class').then(function (classValue) {
            classValue = classValue.split(' ');
            return classValue.indexOf(checkedAllClass) > -1 ? 'checked-all' :
                classValue.indexOf(checkedPartlyClass) > -1 ? 'checked-partly' :
                'unchecked';
        })
    };

    /**
     * Check checkbox to given state
     * @param {boolean} check
     * @returns {!webdriver.promise.Promise}
     */
    this.check = function (check) {
        return _this.getCheckboxState().then(function (isCheckedAll) {
            return _this.checkbox.click().then(function () {
                if (check && isCheckedAll === 'checked-all' || !check && isCheckedAll !== 'checked-all') {
                    return _this.checkbox.click();
                }
            });
        });
    }
}
module.exports = CheckAllCheckbox;
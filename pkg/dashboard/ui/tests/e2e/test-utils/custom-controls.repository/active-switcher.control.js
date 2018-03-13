function ActiveSwitcher(index) {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var containerXPath = "//igz-active-switcher";
    var _this = this;

    index = index || 1;

    this.switcher = control.get(by.xpath("(" + containerXPath + ")[" + index + "]//div[contains(@class, 'igz-active-switcher')]"));

    /**
     * Check active switcher to given state
     * @param {boolean} state
     * @returns {!webdriver.promise.Promise}
     */
    this.switch = function (state) {
        return _this.getText().then(function (switcherState) {
            return switcherState === 'ON';
        }).then(function (isActive) {
            return _this.switcher.click().then(function () {
                if (state === isActive) {
                    return _this.switcher.click();
                }
            });
        });
    };

    /**
     * Check whether the all checkboxes checked value match the given value
     * @returns {!webdriver.promise.Promise.<string>}
     */
    this.getText = function () {
        return control.all(by.xpath(_this.switcher.getLocator() + "//div[contains(@class, 'igz-switcher-text')]")).getVisibleTextList()
            .then(function (visibleElement) {
                return visibleElement[0];
            });
    };

    /**
     * Check whether the all checkboxes checked value match the given value
     * @returns {!webdriver.promise.Promise.<string>}
     */
    this.isActive = function (state) {
        return _this.getText().then(function (switcherState) {
            return switcherState === 'ON';
        }).then(function (isActive) {
            return state === isActive;
        });
    };
}
module.exports = ActiveSwitcher;
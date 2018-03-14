function NavigatorTabs() {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var _this = this;

    this.allTabs = control.all(by.xpath("//igz-navigation-tabs/div[contains(@class,'igz-navigation-tabs')]/div[contains(@class,'navigation-tab')]"));
    this.navigatorTab = function (tabName) {
        return control.get(by.xpath("//igz-navigation-tabs/div[contains(@class,'igz-navigation-tabs')]/div[contains(@class,'navigation-tab')and(contains(text(),'" + tabName + "'))]"))
    };

    /**
     * Returns an array of navigator tabs list
     * @returns {!webdriver.promise.Promise.<Array>}
     */
    this.getNavigatorTabsList = function () {
        return _this.allTabs.getTextList();
    };

    /**
     * Check whether the given tab active
     * @param {string} tabName
     * @returns {!webdriver.promise.Promise.<boolean>}
     */
    this.isTabActive = function (tabName) {
        return _this.navigatorTab(tabName).isAttributeContainingValue('class', 'active');
    }
}
module.exports = NavigatorTabs;
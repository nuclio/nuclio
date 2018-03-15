function DropdownPermissions(parentElementXPath, index) {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var containerXPath;
    var _this = this;

    index = index || 1;
    containerXPath = "(" + parentElementXPath + "//igz-permissions-dropdown)[" + index + "]";

    // controls
    this.dropdownField = control.get(by.xpath(containerXPath + "//div[contains(@class, 'permissions-dropdown-field')]"));
    this.dropdownOptionsContainer = control.get(by.xpath(containerXPath + "//div[contains(@class, 'permissions-dropdown-container')]"));
    this.dropdownOptionsName = control.all(by.xpath(containerXPath + "//div[contains(@class, 'permissions-dropdown-container')]//ul[@class='list']/li/span"));
    this.dropdownOptionByText = function (text) {
        return control.get(by.xpath(containerXPath + "//div[contains(@class, 'permissions-dropdown-container')]//li/span[contains(text(), '" + text + "')]/.."));
    };
    this.dropdownOptionByIndex = function (index) {
        return control.get(by.xpath(containerXPath + "//div[contains(@class, 'permissions-dropdown-container')]//ul[@class='list']/li[" + index + "]"));
    };

    // methods
    /**
     * Returns dropdown selected value
     * @returns {!webdriver.promise.Promise.<string>}
     */
    this.getSelectedDropdownValue = function () {
        return _this.dropdownField.getText()
    };
}
module.exports = DropdownPermissions;
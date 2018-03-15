function DropdownInputtable(parentElementXPath, index) {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var containerXPath;
    var _this = this;

    index = index || 1;
    containerXPath = "(" + parentElementXPath + "//igz-container-wizard-typing-dropdown)[" + index + "]";

    // controls
    this.dropdownInputField = control.get(by.xpath(containerXPath + "//input[contains(@class, 'dropdown-input')]"));
    this.dropdownInputVlidateLine = control.get(by.xpath(containerXPath + "//input[contains(@class, 'dropdown-input')]"));
    this.dropdownField = control.get(by.xpath(containerXPath + "//div[contains(@class, 'container-wizard-dropdown-field')]"));
    this.dropdownOptionsContainer = control.get(by.xpath(containerXPath + "//div[contains(@class, 'container-wizard-dropdown-container ')]"));
    this.dropdownOptions = control.all(by.xpath(containerXPath + "//div[contains(@class, 'container-wizard-dropdown-container ')]//ul[@class='list']/li"));
    this.dropdownOptionsName = control.all(by.xpath(containerXPath + "//div[contains(@class, 'container-wizard-dropdown-container ')]//ul[@class='list']/li/span[1]"));
    this.dropdownOptionByText = function (text) {
        return control.get(by.xpath(containerXPath + "//div[contains(@class, 'container-wizard-dropdown-container ')]//ul[@class='list']/li//span[contains(text(), '" + text + "')]/.."));
    };
    this.dropdownOptionByIndex = function (index) {
        return control.get(by.xpath(containerXPath + "//div[contains(@class, 'container-wizard-dropdown-container ')]//ul[@class='list']/li[" + index + "]"));
    };

    // methods
    /**
     * Returns dropdown selected value
     * @returns {!webdriver.promise.Promise.<string>}
     */
    this.getSelectedDropdownValue = function () {
        return _this.dropdownInputField.getAttribute('value')
    };
}
module.exports = DropdownInputtable;
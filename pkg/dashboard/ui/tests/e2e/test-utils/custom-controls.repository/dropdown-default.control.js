function DropdownDefault(parentElementXPath, index) {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var containerXPath;
    var _this = this;

    index = index || 1;
    containerXPath = "(" + parentElementXPath + "//igz-default-dropdown)[" + index + "]";

    // controls
    this.container = control.get(by.xpath(containerXPath + "/div[contains(@class, 'default-dropdown')]"));
    this.allDropdownSelectedValues = control.all(by.xpath(parentElementXPath + "//igz-default-dropdown//div[contains(@class, 'default-dropdown-field')]/div[@class='dropdown-selected-item']/input"));
    this.dropdownField = control.get(by.xpath(containerXPath + "//div[contains(@class, 'default-dropdown-field')]"));
    this.dropdownSelectedValue = control.get(by.xpath(containerXPath + "//div[contains(@class, 'default-dropdown-field')]/div[@class='dropdown-selected-item']//input"));
    this.dropdownSelectedValueCounterBadge = control.get(by.xpath(containerXPath + "//div[contains(@class, 'default-dropdown-field')]/div[@class='dropdown-selected-item']//div[contains(@class, 'count')]"));
    this.dropdownOptionsContainer = control.get(by.xpath(containerXPath + "//div[contains(@class, 'default-dropdown-container')]"));
    this.dropdownOptions = control.all(by.xpath(containerXPath + "//div[contains(@class, 'default-dropdown-container')]//ul[@class='list']/li[not(contains(@class,'ng-hide'))]"));
    this.dropdownOptionsName = control.all(by.xpath(containerXPath + "//div[contains(@class, 'default-dropdown-container')]//ul[@class='list']/li//span[1]"));
    this.dropdownOptionsDescription = control.all(by.xpath(containerXPath + "//div[contains(@class, 'default-dropdown-container')]//ul[@class='list']/li//span[2]"));
    this.dropdownOptionByText = function (text) {
        return control.get(by.xpath(containerXPath + "//div[contains(@class, 'default-dropdown-container')]//ul[@class='list']/li//span[text()='" + text + "']/.."));
    };
    this.dropdownOptionByIndex = function (index) {
        return control.get(by.xpath(containerXPath + "//div[contains(@class, 'default-dropdown-container')]//ul[@class='list']/li[" + index + "]"));
    };
    this.bottomLink = control.get(by.xpath(containerXPath + "//a[contains(@class,'add-button')]"));
    this.validatorLine = control.get(by.xpath(containerXPath + "//div[contains(@class, '-status-line')]"));

    // methods
    /**
     * Check whether the all dropdown selected values
     * @returns {!webdriver.promise.Promise.<string>}
     */
    this.getAllThisDropdownSelectedValues = function () {
        return _this.allDropdownSelectedValues.getAttributesList('value');
    };

    /**
     * Returns dropdown selected value
     * @returns {!webdriver.promise.Promise.<string>}
     */
    this.getSelectedDropdownValue = function () {
        return _this.dropdownSelectedValue.getAttribute('value');
    };

    /**
     * Check whether the validator border invalid
     * @param {boolean} isInvalid
     * @returns {!webdriver.promise.Promise.<boolean>}
     */
    this.isValidatorBorderInvalid = function (isInvalid) {
        return _this.container.isAttributeContainingValue('class', 'invalid').then(function (invalid) {
            return isInvalid === invalid;
        });
    };
}
module.exports = DropdownDefault;
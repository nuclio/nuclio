function ActionMenu(index) {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var containerXPath;

    index = index || 1;

    containerXPath = "(//igz-action-menu)[" + index + "]";

    this.menuButton = control.get(by.xpath(containerXPath + "//div[contains(@class, 'menu-button')]"));
    this.menuDropdown = control.get(by.xpath(containerXPath + "//div[contains(@class, 'menu-dropdown')]"));
    this.menuDropdownActions = control.all(by.xpath(containerXPath + "//igz-action-item//div[contains(@class, 'igz-action-item')]"));
    this.menuDropdownActionByIndex = function (index) {
        return control.get(by.xpath("(" + containerXPath + "//igz-action-item//div[contains(@class, 'igz-action-item')])[" + index + "]"));
    };
    this.menuDropdownActionByText = function (text) {
        return control.get(by.xpath(containerXPath + "//igz-action-item//div[contains(@class, 'action-label') and (text()='" + text + "')]"));
    };
    this.menuDropdownShortcuts = control.all(by.xpath(containerXPath + "//div[contains(@class, 'shortcuts-item')]"));
    this.menuDropdownShortcutsByIndex = function (index) {
        return control.get(by.xpath("(" + containerXPath + "//div[contains(@class, 'shortcuts-item')])[" + index + "]"));
    };
    this.menuDropdownShortcutsByText = function (text) {
        return control.get(by.xpath(containerXPath + "//div[contains(@class, 'shortcuts-item') and (text()='" + text + "')]"));
    };
}

module.exports = ActionMenu;
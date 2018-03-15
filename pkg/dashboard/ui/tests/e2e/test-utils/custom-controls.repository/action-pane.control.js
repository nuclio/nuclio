function ActionPanel(index) {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var containerXPath;

    index = index || 1;
    containerXPath = "(//*[contains(@class, 'action-panel')])[" + index + "]";

    // controls
    this.container = control.get(by.xpath(containerXPath));
    this.buttons = control.all(by.xpath(containerXPath + "//igz-action-item//div[contains(@class,'action-icon')]"));
    this.moreIcon = control.get(by.xpath(containerXPath + "//igz-action-item-more//div[contains(@class, 'igz-icon-context-menu')]"));
    this.emptyContainer = control.get(by.xpath(containerXPath + "//div[contains(@class, 'empty')]"));
    this.buttonTooltip = control.get(by.xpath(containerXPath + "//igz-action-item//div[contains(@class, 'tooltip-inner')]"));

    /**
     * Returns a action pane button by given button index
     * @param {number} button_index
     * @returns {Control}
     */
    this.buttonByIndex = function (button_index) {
        return control.get(by.xpath("(" + containerXPath + "//igz-action-item//div[contains(@class,'action-icon')])[" + button_index + "]"));
    };

    /**
     * Returns a action pane button by given index
     * @param {number} index
     * @returns {Control}
     */
    this.buttonByLabel = function (label) {
        return control.get(by.xpath(containerXPath + "//igz-action-item//div[contains(@class, 'action-label') and (text()='" + label + "')]/.."));
    };

    /**
     * Returns a action pane button by given index
     * @param {string} text
     * @returns {Control}
     */
    this.buttonByClass = function (text) {
        return control.get(by.xpath(containerXPath + "//igz-action-item//div[contains(@class, '" + text + "')]/.."));
    };

    // methods
    /**
     * Returns an array of buttons title values
     * @returns {!webdriver.promise.Promise.<boolean>}
     */
    this.getButtonsTitlesList = function () {
        return control.all(by.xpath(containerXPath + "/*[contains(@class, 'actions-list')]/igz-action-item//div[contains(@class, 'action-label')]")).getAttributesList('textContent');
    };

    /**
     * Returns an array of more buttons title values
     * @returns {!webdriver.promise.Promise.<boolean>}
     */
    this.getMoreButtonsTitlesList = function () {
        return control.all(by.xpath(containerXPath + "/*[contains(@class, 'actions-list')]/igz-action-item-more//div[contains(@class, 'action-label')]")).getAttributesList('textContent');
    };
}
module.exports = ActionPanel;
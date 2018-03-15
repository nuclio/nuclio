function RefreshActionItem() {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var mouseUtils = e2e_imports.testUtil.mouseUtils();
    var _this = this;

    // Controls
    this.refreshButton = control.get(by.xpath("//div[contains(@class,'igz-icon-refresh')]"));
    this.tooltip = control.get(by.xpath("//div[contains(@class,'tooltip-inner')]"));

    // Methods
    /**
     * Get tool tip title
     * @returns {!webdriver.promise.Promise.<string>}
     */
    this.getTooltipTitle = function () {
        return mouseUtils.move(_this.refreshButton).then(function () {
            return _this.tooltip.getText();
        });
    };
}
module.exports = RefreshActionItem;

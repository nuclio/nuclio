function InfoPane(index) {
    var browserUtils = e2e_imports.testUtil.browserUtils();
    var control = e2e_imports.testUtil.elementFinderUtils();
    var _this = this;
    var containerXPath;

    index = index || 1;
    containerXPath = "(//div[contains(@class, 'info-page-pane ')])[" + index + "]";

    this.container = control.get(by.xpath(containerXPath));
    this.infoPaneButton = control.get(by.xpath("//igz-actions-panes//div[contains(@class,'igz-action-item last-item')]"));
    this.title = control.get(by.xpath(containerXPath + "//div[contains(@class,'info-pane-title ')]"));
    this.titleInput = control.get(by.xpath("(" + containerXPath + "//input[contains(@class,'info-pane-input')])[1]"));
    this.titleDescription = control.get(by.xpath("(" + containerXPath + "//input[contains(@class,'info-pane-input')])[2]"));
    this.closeButton = control.get(by.xpath(containerXPath + "//div[contains(@class, 'close-button')]"));
    this.errorMessage = control.get(by.xpath(containerXPath + "//span[contains(@class,'error-text')][not(contains(@class,'hide'))]"));

    /**
     * Type Info Pane title
     * @param {string} name
     * @returns {!webdriver.promise.Promise}
     */
    this.enterTitleName = function (name) {
        return _this.titleInput.click()
            .then(function () {
                return _this.titleInput.clear();
            })
            .then(function () {
                return _this.titleInput.sendKeys(name);
            })
    };

    /**
     * Type Info Pane description
     * @param {string} description
     * @returns {!webdriver.promise.Promise}
     */
    this.enterTitleDescription = function (description) {
        return _this.titleDescription.click()
            .then(function () {
                return _this.titleDescription.clear();
            })
            .then(function () {
                return _this.titleDescription.sendKeys(description);
            })
    };

    /**
     * Scroll Info Pane to the top
     * @returns {!webdriver.promise.Promise}
     */
    this.scrollToTop = function () {
        return browserUtils.executeScript("$('.info-page-pane .mCSB_container')[0].style.top = '0px'");
    };

    /**
     * scroll Info Pane to element
     * @param {Control} el
     * @returns {!webdriver.promise.Promise}
     */
    this.scrollToElement = function (el) {
        return el.getLocation()
            .then(function (location) {
                return browserUtils.executeScript("$('.info-page-pane .mCSB_container')[0].style.top = '-" + (location.y - 150) + "px'");
            });
    };
}
module.exports = InfoPane;
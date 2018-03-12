function FilterPane(index) {
    var browserUtils = e2e_imports.testUtil.browserUtils();
    var control = e2e_imports.testUtil.elementFinderUtils();
    var scrollScript = "$('.info-page-filters-wrapper .mCSB_container')[0].style.top = ";
    var containerXPath;
    var _this = this;

    index = index || 1;
    containerXPath = "(//igz-info-page-filters)[" + index + "]";

    this.container = control.get(by.xpath(containerXPath));
    this.filterPanel = control.get(by.xpath(containerXPath + "//div[contains(@class,'info-page-filters ')]"));
    this.filterBookmark = control.get(by.xpath(containerXPath + "/..//div[contains(@class, 'igz-icon-filter')]"));
    this.filterBookmarkCounter = control.get(by.xpath(containerXPath + "/..//div[contains(@class, 'igz-icon-filter')]/following-sibling::span[contains(@class,'filter-counter')]"));
    this.closeButton = control.get(by.xpath(containerXPath + "//div[contains(@class, 'close-button')]"));
    this.searchInput = function (inputIndex) {
        inputIndex = inputIndex || 1;
        return control.get(by.xpath('(' + containerXPath + "//input[contains(@class,'input')])[" + inputIndex + "]"));
    };
    this.searchInputClearButton = function (inputIndex) {
        inputIndex = inputIndex || 1;
        return control.get(by.xpath('(' + containerXPath + "//input[contains(@class,'input')])[" + inputIndex + "]/following-sibling::span[contains(@class,'clear')]"));
    };
    this.filterLabels = control.all(by.xpath(containerXPath + "//div[contains(@class,'block-title')]"));
    this.filterLabel = function (index) {
        return control.get(by.xpath('(' + containerXPath + "//div[contains(@class,'block-title')])[" + index + "]"));
    };
    this.filterLabelCounter = function (index) {
        return control.get(by.xpath('(' + containerXPath + "//div[contains(@class,'block-title')])[" + index + "]//span[contains(@class,'filter-counter')]"));
    };
    this.applyFiltersButton = control.get(by.xpath(containerXPath + "//button[text()='Apply']"));
    this.resetFiltersButton = control.get(by.xpath(containerXPath + "//button[text()='Reset']"));
    this.errorMessage = control.get(by.xpath("//div[contains(@class,'search-input-not-found')][not(contains(@class,'hide'))]/descendant-or-self::*[not(*)][1]"));

    this.clickFilterIcon = function () {
        return _this.filterBookmark.click().then(function () {
            return browserUtils.waitForCondition('filters pane opens/closes', function () {
                return _this.filterPanel.isAttributeContainingValue('class', 'ng-animate').then(function (isContaining) {
                    return isContaining === false;
                })
            }, 3000);
        });
    };

    this.openFilterPane = function () {
        return _this.filterBookmark.click().then(function () {
            return browserUtils.waitForCondition('filters pane opens', function () {
                return _this.filterPanel.getAttribute('class').then(function (attribute) {
                    return attribute == 'info-page-filters info-page-filters-shown';
                })
            }, 3000);
        });
    };

    this.closeFilterPane = function () {
        return _this.filterBookmark.click().then(function () {
            return browserUtils.waitForCondition('filters pane closes', function () {
                return _this.filterPanel.getAttribute('class').then(function (attribute) {
                    return attribute == 'info-page-filters ng-hide';
                })
            }, 3000);
        });
    };

    this.fillSearchInput = function (text) {
        return _this.searchInput.click().then(function () {
            return _this.searchInput.clear();
        }).then(function () {
            return _this.searchInput.sendKeys(text);
        })
    };

    /**
     * Scroll Filter Pane to the top
     * @returns {!webdriver.promise.Promise}
     */
    this.scrollToTop = function () {
        return browserUtils.executeScript(scrollScript + "'0px'");
    };

    /**
     * Scroll Filter Pane to element
     * @param {Control} el
     * @returns {!webdriver.promise.Promise}
     */
    this.scrollToElement = function (el) {
        return el.getLocation()
            .then(function (location) {
                return browserUtils.executeScript(scrollScript + "'-" + (location.y - 200) + "px'");
            });
    };

    /**
     * Get filter labels
     * @returns {!webdriver.promise.Promise.<Array>}
     */
    this.getLabelsList = function () {
        return _this.filterLabels.getTextList();
    };

    /**
     * Check whether section is collapsed
     * @param {index} number
     * @returns {!webdriver.promise.Promise.<boolean>}
     */
    this.isFilterSectionCollapsed = function (index) {
        return _this.filterLabel(index).isAttributeContainingValue('class', ' collapsed');
    };

    /**
     * Click on filter section
     * @param {index} number
     * @returns {!webdriver.promise.Promise.<void>}
     */
    this.clickFilterSection = function (index) {
        return _this.filterLabel(index).click();
    };
}
module.exports = FilterPane;
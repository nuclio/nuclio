function InfoPaneSections() {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var sectionBlockXPath = "//div[not(@class='tab-pane ng-scope')]/div[contains(@class,'info-pane-tab-content')][not(contains(@class,'hide'))]";
    var _this = this;

    this.sectionLabels = control.all(by.xpath(sectionBlockXPath + "//div[contains(@class,'collapsed-block-title')]"));
    this.sectionLabelByText = function (text) {
        return control.get(by.xpath(sectionBlockXPath + "//div[contains(@class,'collapsed-block-title')][contains(text(),'" + text + "')]"));
    };
    this.sectionLabelByIndex = function (index) {
        return control.get(by.xpath("(" + sectionBlockXPath + "//div[contains(@class,'collapsed-block-title')])[" + index + "]"));
    };

    /**
     * Return section by title
     * @param {string} title
     * @returns {SectionByTitle}
     */
    this.getSectionByTitle = function (title) {
        return new SectionByTitle(title);
    };

    function SectionByTitle(title) {

        /**
         * Collapse Info Pane section
         * @param {boolean} collapse
         * @returns {!webdriver.promise.Promise}
         */
        this.collapse = function (collapse) {
            return _this.sectionLabelByText(title).click().then(function () {
                return _this.sectionLabelByText(title).isAttributeContainingValue('class', ' collapsed')
            }).then(function (isCollapsed) {
                if (isCollapsed != collapse) {
                    return _this.sectionLabelByText(title).click();
                }
            });
        };

        /**
         * Check whether section is collapsed
         * @returns {!webdriver.promise.Promise.<boolean>}
         */
        this.isCollapsed = function () {
            return _this.sectionLabelByText(title).isAttributeContainingValue('class', ' collapsed');
        };
    }

    /**
     * Return section by title
     * @param {number} index
     * @returns {SectionByIndex}
     */
    this.getSectionByIndex = function (index) {
        return new SectionByIndex(index);
    };

    function SectionByIndex(index) {

        /**
         * Collapse Info Pane section
         * @param {boolean} collapse
         * @returns {!webdriver.promise.Promise}
         */
        this.collapse = function (collapse) {
            return _this.sectionLabelByIndex(index).click().then(function () {
                return _this.sectionLabelByIndex(index).isAttributeContainingValue('class', ' collapsed')
            }).then(function (isCollapsed) {
                if (isCollapsed != collapse) {
                    return _this.sectionLabelByIndex(index).click();
                }
            });
        };

        /**
         * Check whether section is collapsed
         * @returns {!webdriver.promise.Promise.<boolean>}
         */
        this.isCollapsed = function () {
            return _this.sectionLabelByIndex(index).isAttributeContainingValue('class', ' collapsed');
        };
    }
}
module.exports = InfoPaneSections;
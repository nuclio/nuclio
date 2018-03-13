function TableHeader(index) {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var mouseUtils = e2e_imports.testUtil.mouseUtils();
    var customControl = e2e_imports.testUtil.customControlsRepository();
    var _this = this;
    index = index || 1;
    var containerXPath = "(//div[contains(@class, 'table-header')])[" + index + "]";

    // controls
    this.tableHeader = control.get(by.xpath(containerXPath + "/div[contains(@class,'common-table-cells-container')]"));
    this.checkAllCheckbox = customControl.checkAllCheckbox(index);
    this.columns = control.all(by.xpath(containerXPath + "//div[contains(concat(' ', @class, ' '),' common-table-cell ')][not(div[contains(@class, 'check-item')])][not(contains(@class, 'actions-menu'))]"));
    this.columnByIndex = function (index) {
        return control.get(by.xpath("(" + containerXPath + "//div[contains(concat(' ', @class, ' '),' common-table-cell ')][not(div[contains(@class, 'check-item')])][not(contains(@class, 'actions-menu'))])[" + index + "]"));
    };
    this.resizeBlock = function (index) {
        return control.get(by.xpath("(//div[contains(@class,'resize-block')])[" + index + "]"));
    };

    // methods
    /**
     * Sort table items by given Column in ascending order
     * @param {number} index
     * @returns {!webdriver.promise.Promise}
     */
    this.sortInAscendingOrder = function (index) {
        return _this.columnByIndex(index).click().then(function () {
            return _this.columnByIndex(index).isAttributeContainingValue('class', 'reversed').then(function (isReversed) {
                if (isReversed) {
                    return _this.columnByIndex(index).click()
                }
            });
        });
    };

    /**
     * Sort table items by given Column in descending order
     * @param {number} index
     * @returns {!webdriver.promise.Promise}
     */
    this.sortInDescendingOrder = function (index) {
        return _this.columnByIndex(index).click().then(function () {
            return _this.columnByIndex(index).isAttributeContainingValue('class', 'reversed').then(function (isReversed) {
                if (!isReversed) {
                    return _this.columnByIndex(index).click()
                }
            });
        });
    };

    /**
     * Resize column header
     * @param {number} colIndex
     * @param {number} x
     * @returns {!webdriver.promise.Promise.<void>}
     */
    this.resizeColumn = function (colIndex, x) {
        return mouseUtils.dragAndDrop(_this.resizeBlock(colIndex), undefined, _this.columnByIndex(colIndex + 1), {x: x - 6 , y: 1});
    };

    /**
     * Return column width
     * @param {number} column
     * @returns {!webdriver.promise.Promise.<string>}
     */
    this.getColumnWidth = function (column) {
        return _this.columnByIndex(column).getSize().then(function (size) {
            return size.width;
        });
    };

    /**
     * Emulate double click on column header
     * @param {number} column
     * @returns {!webdriver.promise.Promise}
     */
    this.resizeBlockDoubleClick = function (column) {
        return mouseUtils.doubleClick(_this.resizeBlock(column));
    };
}
module.exports = TableHeader;
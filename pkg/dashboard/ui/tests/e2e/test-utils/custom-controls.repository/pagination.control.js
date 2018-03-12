function Pagination(parentElementXPath) {
    parentElementXPath = parentElementXPath ? parentElementXPath : '';
    var control = e2e_imports.testUtil.elementFinderUtils();
    var customControl = e2e_imports.testUtil.customControlsRepository();

    this.PerPageCount = {
        _10: '10',
        _20: '20',
        _30: '30',
        _40: '40',
        _50: '50'
    };

    // controls
    this.paginationSection = control.get(by.xpath(parentElementXPath + "//igz-pagination"));
    this.paginationArticle = control.get(by.xpath(parentElementXPath + "//div[@class='igz-pagination']/div[contains(@class, 'rows-title')]"));
    this.perPageDropdown = customControl.dropdownDefault(parentElementXPath + "//igz-pagination", 1);
    this.paginationJumpToPageArticle = control.get(by.xpath(parentElementXPath + "//div[contains(@class,'jump-to-page ')]/div[contains(@class, 'rows-title title')]"));
    this.paginationJumpToPageInput = customControl.inputFieldValidating(parentElementXPath + "//div[contains(@class,'jump-to-page ')]", 1);
    this.paginationJumpToPageBlock = control.get(by.xpath(parentElementXPath + "//div[contains(@class,'jump-to-page')]"));
    this.paginationJumpToPageRowsTitle = control.get(by.xpath(parentElementXPath + "//div[contains(@class,'jump-to-page')]//div[contains(@class,'rows-title')]"));
    this.paginationPreviousButton = control.get(by.xpath(parentElementXPath + "//div[contains(@class,'to-page-prev')]"));
    this.paginationNextButton = control.get(by.xpath(parentElementXPath + "//div[contains(@class,'to-page-next')]"));
}
module.exports = Pagination;
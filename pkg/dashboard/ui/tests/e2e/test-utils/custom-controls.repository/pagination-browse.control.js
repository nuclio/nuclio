function PaginationBrowse() {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var customControl = e2e_imports.testUtil.customControlsRepository();

    this.PerPageCount = {
        _10: '10',
        _50: '50',
        _100: '100',
        _150: '150'
    };

    // controls
    this.paginationSection = control.get(by.xpath("//igz-browser-pagination"));
    this.paginationArticle = control.get(by.xpath("//igz-browser-pagination//div[@class='rows-title']"));
    this.perPageDropdown = customControl.dropdownDefault("//igz-browser-pagination", 1);
    this.paginationPreviousButton = control.get(by.xpath("//igz-browser-pagination//div[contains(@class,'to-page-prev')]"));
    this.paginationNextButton = control.get(by.xpath("//igz-browser-pagination//div[contains(@class,'to-page-next')]"));
}
module.exports = PaginationBrowse;
function InfoPaneTabs() {
    var control = e2e_imports.testUtil.elementFinderUtils();

    this.tabs = control.all(by.xpath("//div[@class='info-pane-navigation-tabs']/div"));
    this.tab = function (index) {
        return control.get(by.xpath("(//div[@class='info-pane-navigation-tabs']/div)[" + index + "]"));
    };
    this.activeTab = control.get(by.xpath("//div[@class='info-pane-navigation-tabs']/div[contains(@class,'active')]"));
    this.leftArrow = control.get(by.xpath("//div[@class='tabs-slider']/div[contains(@class,'left-arrow')]"));
    this.rightArrow = control.get(by.xpath("//div[@class='tabs-slider']/div[contains(@class,'right-arrow')]"));
}
module.exports = InfoPaneTabs;
function TagsInput(parentElementXPath, index) {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var containerXPath;
    var _this = this;

    index = index || 1;

    containerXPath = "(" + parentElementXPath + "//igz-tags-input)[" + index + "]";

    this.inputField = control.get(by.xpath(containerXPath + "//input"));
    this.dropdownContainer = control.get(by.xpath(containerXPath + "//div[contains(@class, 'dropdown-container')]"));
    this.dropdownItemsList = control.all(by.xpath(containerXPath + "//div[contains(@class, 'dropdown-container')]//div[contains(@class, 'tags-item')][not(contains(@class, 'ng-hide'))]"));
    this.dropdownItemByIndex = function(item_index){
        return control.get(by.xpath("(" + containerXPath + "//div[contains(@class, 'dropdown-container')]//div[contains(@class, 'tags-item')][not(contains(@class, 'ng-hide'))]/span[contains(@class, 'text')])[" + item_index + "]"));
    };
    this.dropdownItemByText = function(item_text){
        return control.get(by.xpath("(" + containerXPath + "//div[contains(@class, 'dropdown-container')]//div[contains(@class, 'tags-item')][not(contains(@class, 'ng-hide'))]/span[contains(@class, 'text')])[text() = '" + item_text + "']"));
    };
    this.selectedItemsList = control.all(by.xpath(containerXPath + "//div[contains(@class, 'selected-tags-container')]//div[contains(@class, 'selected-tags')][not(contains(@class, 'ng-hide'))]"));
    this.selectedItemByIndex = function (item_index) {
        return control.get(by.xpath("(" + containerXPath + "//div[contains(@class, 'selected-tags-container')]//div[contains(@class, 'selected-tags-item')][not(contains(@class, 'ng-hide'))])[" + item_index + "]"));
    };
    this.selectedItemXButtonByIndex = function (item_index) {
        return control.get(by.xpath("(" + containerXPath + "//div[contains(@class, 'selected-tags-container')]//div[contains(@class, 'selected-tags')][not(contains(@class, 'ng-hide'))])[" + item_index + "]/span[@class='igz-icon-close']"));
    };
}
module.exports = TagsInput;
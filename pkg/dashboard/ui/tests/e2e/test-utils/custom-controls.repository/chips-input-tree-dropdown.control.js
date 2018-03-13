function ChipsInputTreeDropdown(parentElementXPath, index) {
    var control = e2e_imports.testUtil.elementFinderUtils();
    var chipsInput = e2e_imports.testUtil.customControlsRepository().chipsInput(parentElementXPath, index);
    var containerXPath;

    index = index || 1;
    containerXPath = "(" + parentElementXPath + "//igz-chips-input)[" + index + "]";

    // controls
    this.container = chipsInput.container;
    this.inputField = chipsInput.inputField;
    this.chipsList = chipsInput.chipsList;
    this.chipXButtonByIndex = chipsInput.chipXButtonByIndex;
    this.treeDropdownContainer = control.get(by.xpath(containerXPath + "//div[contains(@class,'tree-dropdown')]"));
    this.suggestedChipsList = control.all(by.xpath(containerXPath + "//div[@class='tree-label']/span"));
    this.suggestedExpandedChipsList = control.all(by.xpath(containerXPath + "//li[contains(@class,'tree-expanded')]//div[@class='tree-label']/span"));
    this.suggestedChipCheckboxByIndex = function (chipIndex) {
        return control.get(by.xpath("(" + containerXPath + "//span[contains(@class,'path-checkbox')])[" + chipIndex + "]"));
    };
    this.suggestedChipLabelByIndex = function (chipIndex) {
        return control.get(by.xpath("(" + containerXPath + "//div[contains(@class,'tree-label')]/span)[" + chipIndex + "]"));
    };
    this.suggestedChipExpandIconByIndex = function (chipIndex) {
        return control.get(by.xpath("(" + containerXPath + "//i[@class='tree-branch-head'])[" + chipIndex + "]"));
    };

    // methods
    this.typeBackspaceTwice = chipsInput.typeBackspaceTwice;
    this.isInputFieldInvalid = chipsInput.isInputFieldInvalid;
    this.typeToChipsInput = chipsInput.typeToChipsInput;
}
module.exports = ChipsInputTreeDropdown;
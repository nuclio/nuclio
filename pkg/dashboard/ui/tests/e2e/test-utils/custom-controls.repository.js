function CustomControlRepository() {

    /**
     * Returns a custom controls that helps to work with action checkbox elements
     * @param {string} parentElementXPath
     * @param {number} index
     * @returns {Checkbox}
     */
    this.actionCheckbox = function (parentElementXPath, index) {
        var ActionCheckbox = require(e2e_root + '/test-utils/custom-controls.repository/action-checkbox.control.js');
        return new ActionCheckbox(parentElementXPath, index);
    };

    /**
     * Returns a custom controls that helps to work with action menu elements
     * data-igz-action-menu
     * @param {number | undefined} index
     * @returns {ActionMenu}
     */
    this.actionMenu = function (index) {
        var ActionMenu = require(e2e_root + '/test-utils/custom-controls.repository/action-menu.control.js');
        return new ActionMenu(index);
    };

    /**
     * Returns a custom controls that helps to work with action pane elements
     * data-igz-action-panel
     * @param {number | undefined} index
     * @returns {ActionPane}
     */
    this.actionsPanel = function (index) {
        var ActionPane = require(e2e_root + '/test-utils/custom-controls.repository/action-pane.control.js');
        return new ActionPane(index);
    };

    /**
     * Returns a custom controls that helps to work with action switcher elements
     * data-igz-active-switcher
     * @returns {ActiveSwitcher}
     */
    this.activeSwitcher = function (index) {
        var ActiveSwitcher = require(e2e_root + '/test-utils/custom-controls.repository/active-switcher.control.js');
        return new ActiveSwitcher(index);
    };

    /**
     * Returns a custom controls that helps to work with filter pane elements
     * data-igz-info-page-filters
     * @param {number | undefined} index
     * @returns {FilterPane}
     */
    this.filterPane = function (index) {
        var FilterPane = require(e2e_root + '/test-utils/custom-controls.repository/filter-pane.control.js');
        return new FilterPane(index);
    };

    /**
     * Returns a custom controls that helps to work with chips input element
     * //igz-chips-input
     * @returns {ChipsInput}
     */
    this.chipsInput = function (parentElementXPath, index) {
        var ChipsInput = require(e2e_root + '/test-utils/custom-controls.repository/chips-input.control.js');
        return new ChipsInput(parentElementXPath, index);
    };

    /**
     * Returns a custom controls that helps to work with chips input element that contains tree dropdown
     * //igz-chips-input
     * @returns {ChipsInputTreeDropdown}
     */
    this.chipsInputTreeDropdown = function (parentElementXPath, index) {
        var ChipsInputTreeDropdown = require(e2e_root + '/test-utils/custom-controls.repository/chips-input-tree-dropdown.control.js');
        return new ChipsInputTreeDropdown(parentElementXPath, index);
    };

    /**
     * Returns a custom controls that helps to work with date picker element
     * //*[@datepicker-popup-wrap]
     * @returns {DatePicker}
     */
    this.datePicker = function (parentElementXPath, index) {
        var DatePicker = require(e2e_root + '/test-utils/custom-controls.repository/date-picker.control.js');
        return new DatePicker(parentElementXPath, index);
    };

    /**
     * Returns a custom controls that helps to work with dialog box element
     * //*[contains(@class, 'ngdialog')]
     * @returns {DialogBox}
     */
    this.dialogBox = function (index) {
        var DialogBox = require(e2e_root + '/test-utils/custom-controls.repository/dialog-box.control.js');
        return new DialogBox(index);
    };

    /**
     * Returns a custom controls that helps to work with checkbox elements
     * @param {string} parentElementXPath
     * @param {?number} index
     * @returns {Checkbox}
     */
    this.radiobuttonGroup = function (parentElementXPath, index) {
        var RadiobuttonGroup = require(e2e_root + '/test-utils/custom-controls.repository/radiobutton-group.control.js');
        return new RadiobuttonGroup(parentElementXPath, index);
    };

    /**
     * Returns a custom controls that helps to work with checkbox elements
     * @param {string} parentElementXPath
     * @param {?number} index
     * @returns {Checkbox}
     */
    this.checkbox = function (parentElementXPath, index) {
        var Checkbox = require(e2e_root + '/test-utils/custom-controls.repository/checkbox.control.js');
        return new Checkbox(parentElementXPath, index);
    };

    /**
     * Returns a custom controls that helps to work with 'Check All' checkbox elements
     * @param {?number} index
     * @returns {CheckAllCheckbox}
     */
    this.checkAllCheckbox = function (index) {
        var CheckAllCheckbox = require(e2e_root + '/test-utils/custom-controls.repository/check-all-checkbox.control.js');
        return new CheckAllCheckbox(index);
    };

    /**
     * Returns a custom controls that helps to work with inputtable dropdown elements
     * @param {string} parentElementXPath
     * @param {?number} index
     * @returns {DropdownInputtable}
     */
    this.dropdownInputtable = function (parentElementXPath, index) {
        var DropdownInputtable = require(e2e_root + '/test-utils/custom-controls.repository/dropdown-inputtable.control.js');
        return new DropdownInputtable(parentElementXPath, index);
    };

    /**
     * Returns a custom controls that helps to work with permissions dropdown elements
     * data-igz-permissions-dropdown
     * @param {string} parentElementXPath
     * @param {?number} index
     * @returns {DropdownPermissions}
     */
    this.dropdownPermissions = function (parentElementXPath, index) {
        var DropdownPermissions = require(e2e_root + '/test-utils/custom-controls.repository/dropdown-permissions.control.js');
        return new DropdownPermissions(parentElementXPath, index);
    };

    /**
     * Returns a custom controls that helps to work with priority dropdown elements
     * data-igz-priority-dropdown
     * @param {string} parentElementXPath
     * @param {?number} index
     * @returns {DropdownPriority}
     */
    this.dropdownPriority = function (parentElementXPath, index) {
        var DropdownPriority = require(e2e_root + '/test-utils/custom-controls.repository/dropdown-priority.control.js');
        return new DropdownPriority(parentElementXPath, index);
    };

    /**
     *
     * Returns a custom controls that helps to work with dropdown elements with scrollable options container
     * data-igz-default-dropdown
     * @param {string} parentElementXPath
     * @param {number|string|undefined} index
     * @returns {DropdownScrollable}
     */
    this.dropdownDefault = function (parentElementXPath, index) {
        var DropdownDefault = require(e2e_root + '/test-utils/custom-controls.repository/dropdown-default.control.js');
        return new DropdownDefault(parentElementXPath, index);
    };

    /**
     * Returns a custom controls that helps to work with underlined input field elements
     * @param {string} parentElementXPath
     * @param {number|string|undefined} index
     * @returns {InputFieldValidating}
     */
    this.inputFieldElastic = function (parentElementXPath, index) {
        var InputFieldElastic = require(e2e_root + '/test-utils/custom-controls.repository/input-field-elastic.control.js');
        return new InputFieldElastic(parentElementXPath, index);
    };

    /**
     * Returns a custom controls that helps to work with underlined input field elements
     * @param {string} parentElementXPath
     * @param {?number} index
     * @returns {InputFieldValidating}
     */
    this.inputFieldValidating = function (parentElementXPath, index) {
        var InputFieldValidating = require(e2e_root + '/test-utils/custom-controls.repository/input-field-validating.control.js');
        return new InputFieldValidating(parentElementXPath, index);
    };

    /**
     * Returns a custom controls that helps to work with underlined digits input field elements
     * @param {string} parentElementXPath
     * @param {?number} index
     * @returns {InputFieldDigitsValidating}
     */
    this.inputFieldDigitsOnly = function (parentElementXPath, index) {
        var InputFieldDigitsValidating = require(e2e_root + '/test-utils/custom-controls.repository/input-field-digits-validating.control.js');
        return new InputFieldDigitsValidating(parentElementXPath, index);
    };

    /**
     * Returns a custom controls that helps to work with underlined input field elements
     * @param {string} parentElementXPath
     * @param {?number} index
     * @returns {InputFieldValidating}
     */
    this.inputFieldNumber = function (parentElementXPath, index) {
        var InputFieldNumber = require(e2e_root + '/test-utils/custom-controls.repository/input-field-number.control.js');
        return new InputFieldNumber(parentElementXPath, index);
    };

    /**
     * Returns a custom controls that helps to work with Info Pane elements
     * @returns {InfoPane}
     */
    this.infoPane = function (index) {
        var InfoPane = require(e2e_root + '/test-utils/custom-controls.repository/info-pane.control.js');
        return new InfoPane(index);
    };

    /**
     * Returns a custom controls that helps to work with Info Pane Tabs elements
     * @returns {InfoPaneTabsContainer}
     */
    this.infoPaneTabsContainer = function () {
        var InfoPaneTabsContainer = require(e2e_root + '/test-utils/custom-controls.repository/info-pane-tabs.control.js');
        return new InfoPaneTabsContainer();
    };

    /**
     * Returns a custom controls that helps to work with Info Pane Sections elements
     * @returns {InfoPaneSections}
     */
    this.infoPaneSections = function () {
        var InfoPaneSections = require(e2e_root + '/test-utils/custom-controls.repository/info-pane-sections.control.js');
        return new InfoPaneSections();
    };

    /**
     * Returns a custom controls that helps to work with Refresh button elements
     * @returns {RefreshButton}
     */
    this.refreshActionItem = function () {
        var RefreshActionItem = require(e2e_root + '/test-utils/custom-controls.repository/refresh-action-item.control.js');
        return new RefreshActionItem();
    };

    /**
     * Returns a custom controls that helps to work with Slider elements
     * @returns {Slider}
     */
    this.slider = function (parentElementXPath, index) {
        var Slider = require(e2e_root + '/test-utils/custom-controls.repository/slider.control.js');
        return new Slider(parentElementXPath, index);
    };

    /**
     * Returns a custom controls that helps to work with Navigator tabs elements
     * @returns {NavigatorTabs}
     */
    this.navigatorTabsContainer = function () {
        var NavigatorTabs = require(e2e_root + '/test-utils/custom-controls.repository/navigator-tabs.control.js');
        return new NavigatorTabs();
    };

    /**
     * Returns a custom controls that helps to work with table header elements
     * //div[contains(@class, 'table-header')]
     * @param {?number} index
     * @returns {TableHeader}
     */
    this.tableHeader = function (index) {
        var TableHeader = require(e2e_root + '/test-utils/custom-controls.repository/table-header.control.js');
        return new TableHeader(index);
    };

    /**
     * Returns a custom controls that helps to work with tags input elements
     * //*[@data-igz-tags-input]
     * @param {string} parentElementXPath
     * @param {?number} index
     * @returns {TagsInput}
     */
    this.tagsInput = function (parentElementXPath,index) {
        var TagsInput = require(e2e_root + '/test-utils/custom-controls.repository/tags-input.control.js');
        return new TagsInput(parentElementXPath,index);
    };

    /**
     * Returns a custom controls that helps to work pagination elements
     * @param {string} parentElementXPath
     * @returns {Pagination}
     */
    this.pagination = function (parentElementXPath) {
        var Pagination = require(e2e_root + '/test-utils/custom-controls.repository/pagination.control.js');
        return new Pagination(parentElementXPath);
    };

    /**
     * Returns a custom controls that helps to work pagination elements
     * @returns {PaginationBrowse}
     */
    this.paginationBrowse = function () {
        var PaginationBrowse = require(e2e_root + '/test-utils/custom-controls.repository/pagination-browse.control.js');
        return new PaginationBrowse();
    };
}

module.exports = CustomControlRepository;
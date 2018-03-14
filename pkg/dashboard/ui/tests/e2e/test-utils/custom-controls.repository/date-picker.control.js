function DatePicker(parentElementXPath) {
    var control = e2e_imports.testUtil.elementFinderUtils();

    // controls
    this.inputField = control.get(by.xpath(parentElementXPath + "//input[contains(@class,'datetimepicker-input')]"));
    this.dateRangeDropdownField = control.get(by.xpath("//ul[@class='options-list']"));
    this.dateRangeDropdown = control.get(by.xpath(parentElementXPath + "//button[contains(@class,'datetimepicker-open-button')]"));
    this.dateRangeDropdownOptions = control.all(by.xpath(parentElementXPath + "//div[contains(@class,'options-dropdown')]//li"));
    this.datePickerApplyButton = control.get(by.xpath(parentElementXPath + "//div[text()='Apply']"));
    this.datePickerTooltip = control.get(by.xpath("//div[contains(@class,'tooltip-inner')]"));
    this.datePickerCancelButton = control.get(by.xpath(parentElementXPath + "//div[contains(text(),'Cancel')]"));
    this.dateRangeDropdownOption = function (index) {
        return control.get(by.xpath("(" + parentElementXPath + "//div[contains(@class,'options-dropdown')]//li)[" + index + "]"));
    };
    this.dateRangeDropdownOptionByText = function (text) {
        return control.get(by.xpath("//div[@class='date-time-picker']//li[text()='" + text + "']"));
    };

    this.datePickerCalendar = function (index) {
        return new DatePickerCalendar(index);
    };

    function DatePickerCalendar(index) {
        var _this = this;
        index = index || 1;
        var containerXPath = "(" + parentElementXPath + "//div[contains(@class,'datepicker-wrapper')])[" + index + "]";

        this.container = control.get(by.xpath(containerXPath + "//div[contains(@class,'datepicker-calendar')]"));
        this.datePickerHeader = control.get(by.xpath(containerXPath + "//strong[@class='ng-binding']"));
        this.datePickerHeaderBackButton = control.get(by.xpath(containerXPath + "//button[contains(@class,'left')]"));
        this.datePickerHeaderForwardButton = control.get(by.xpath(containerXPath + "//button[contains(@class,'right')]"));
        this.datePickerDateByText = function (text) {
            return control.get(by.xpath(containerXPath + "//span[contains(text(),'" + text + "')][not(contains(@class,'text-muted'))]"));
        };
        this.datePickerSelectedDay = control.get(by.xpath(containerXPath + "//button[contains(@class, 'active')]/span"));
        this.datePickerHoursArrowUp = control.get(by.xpath("(" + containerXPath + "//span[@class='glyphicon glyphicon-chevron-up'])[1]"));
        this.datePickerHoursArrowDown = control.get(by.xpath("(" + containerXPath + "//span[@class='glyphicon glyphicon-chevron-down'])[1]"));
        this.datePickerMinutesArrowUp = control.get(by.xpath("(" + containerXPath + "//span[@class='glyphicon glyphicon-chevron-up'])[2]"));
        this.datePickerMinutesArrowDown = control.get(by.xpath("(" + containerXPath + "//span[@class='glyphicon glyphicon-chevron-down'])[2]"));
        this.datePickerHoursInputField = control.get(by.xpath("(" + containerXPath + "//input[contains(@class,'form-control')])[1]"));
        this.datePickerMinutesInputField = control.get(by.xpath("(" + containerXPath + "//input[contains(@class,'form-control')])[2]"));
        this.datePickerTimeInputsWrappers = control.all(by.xpath(containerXPath + "//td[contains(@class, 'form-group')]"));
        this.datePickerAmPmButton = control.get(by.xpath(containerXPath + "//table[contains(@class,'timepicker')]//button[contains(@class,'btn btn-default')]"));

        // methods
        /**
         * Select past date clicking on Date picker
         * @param {string} month_year
         * @param {string} day
         * @returns {!webdriver.promise.Promise}
         */
        this.selectPastDate = function (month_year, day) {
            _this.datePickerHeader.getText().then(function (header) {
                if (!header.includes(month_year)) {
                    _this.datePickerHeaderBackButton.click().then(_this.selectPastDate(month_year, day));
                } else {
                    return _this.datePickerDateByText(day).click();
                }
            })
        };

        /**
         * Select future date clicking on Date picker
         * @param {string} month_year
         * @param {string} day
         * @returns {!webdriver.promise.Promise}
         */
        this.selectFutureDate = function (month_year, day) {
            _this.datePickerHeader.getText().then(function (header) {
                if (header != month_year) {
                    _this.datePickerHeaderForwardButton.click().then(_this.selectFutureDate(month_year, day));
                } else {
                    return _this.datePickerDateByText(day).click();
                }
            })
        };

        /**
         * Click arrow Up on hours
         * @returns {!webdriver.promise.Promise}
         */
        this.clickHoursArrowUp = function () {
            return _this.datePickerHoursArrowUp.click();
        };

        /**
         * Click arrow Down on hours
         * @returns {!webdriver.promise.Promise}
         */
        this.clickHoursArrowDown = function () {
            return _this.datePickerHoursArrowDown.click();
        };

        /**
         * Click arrow Up on minutes
         * @returns {!webdriver.promise.Promise}
         */
        this.clickMinutesArrowUp = function () {
            return _this.datePickerMinutesArrowUp.click();
        };

        /**
         * Click arrow Down on minutes
         * @returns {!webdriver.promise.Promise}
         */
        this.clickMinutesArrowDown = function () {
            return _this.datePickerMinutesArrowDown.click();
        };

        /**
         * Type given text to hours input field
         * @param {string} text
         * @returns {!webdriver.promise.Promise}
         */
        this.enterHours = function (text) {
            return _this.datePickerHoursInputField.click().then(function () {
                return _this.datePickerHoursInputField.clear()
            }).then(function () {
                return _this.datePickerHoursInputField.sendKeys(text)
            });
        };

        /**
         * Type given text to minutes input field
         * @param {string} text
         * @returns {!webdriver.promise.Promise}
         */
        this.enterMinutes = function (text) {
            return _this.datePickerMinutesInputField.click().then(function () {
                return _this.datePickerMinutesInputField.clear()
            }).then(function () {
                return _this.datePickerMinutesInputField.sendKeys(text)
            });
        };

        /**
         * Set AM/PM time clicking on button
         * @param {string} am_pm
         * @returns {!webdriver.promise.Promise}
         */
        this.setAmPmTime = function (am_pm) {
            return _this.datePickerAmPmButton.getText().then(function (time) {
                if (time != am_pm) {
                    _this.datePickerAmPmButton.click();
                }
            })
        };

        /**
         * Check whether the validator border visible
         * @param {boolean} invalid
         * @returns {!webdriver.promise.Promise.<boolean>}
         */
        this.areTimeValidatorBordersInvalid = function (invalid) {
            return _this.datePickerTimeInputsWrappers.areAttributesContainingValue('class', 'invalid', invalid);
        };
    }
}
module.exports = DatePicker;
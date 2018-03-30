(function () {
    'use strict';

    /**
     * compareInputValue: used if there are two field that should be equal (password and confirm password)
     * fieldType: input, textarea or password
     * formObject: object of HTML form
     * hideCounter: should be counter of remaining symbols for the field visible or not
     * inputId: string that should be assigned to id attribute
     * inputModelOptions: custom options for ng-model-options
     * inputName: name attribute of an input
     * inputValue: data model
     * itemBlurCallback: callback for onBlur event
     * itemFocusCallback: callback for onFocus event
     * isDataRevert: should incorrect value be immediately replaced by a previous correct one
     * isDisabled: is input should be disabled
     * isFocused: should input be focused when screen is displayed
     * onlyValidCharacters: allow only that characters which passed regex pattern
     * placeholderText: text that is displayed when input is empty
     * readOnly: is input should be readonly
     * spellcheck: disable spell check for some field, for example input for base64 string
     * updateDataCallback: triggered when input was changed by a user, added whn two-way binding was replased with one-way
     * updateDataField: field name for updateDataCallback
     * validationIsRequired: input can't be empty
     * validationMaxLength: value should be shorter or equal this value
     * validationPattern: validation with regex
     */
    angular.module('iguazio.app')
        .component('igzValidatingInputField', {
            bindings: {
                compareInputValue: '<?',
                enterCallback: '<?',
                fieldType: '@',
                formObject: '<',
                hideCounter: '@?',
                inputIcon: '@',
                inputModelOptions: '<?',
                inputName: '@',
                inputValue: '<',
                isDisabled: '<?',
                isDataRevert: '@?',
                isFocused: '<?',
                itemBlurCallback: '&?',
                itemFocusCallback: '&?',
                onBlur: '&?',
                onlyValidCharacters: '@?',
                placeholderText: '@',
                readOnly: '<?',
                spellcheck: '@?',
                updateDataCallback: '&?',
                updateDataField: '@?',
                validationIsRequired: '@',
                validationMaxLength: '@',
                validationPattern: '<',
                isClearIcon: '<?'
            },
            templateUrl: 'shared-dashboard-controls/components/validating-input-field/validating-input-field.tpl.html',
            controller: IgzValidatingInputFieldController
        });

    function IgzValidatingInputFieldController($element, $timeout, $window, lodash, EventHelperService, FormValidationService) {
        var ctrl = this;

        var defaultInputModelOptions = {
            updateOn: 'default blur',
            debounce: {
                'default': 1000,
                'blur': 0
            },
            allowInvalid: true
        };

        ctrl.data = '';
        ctrl.inputFocused = false;
        ctrl.startValue = '';

        ctrl.$onInit = onInit;
        ctrl.$onChanges = onChanges;
        ctrl.$onDestroy = onDestroy;
        ctrl.$postLink = postLink;

        ctrl.getRemainingSymbolsCounter = getRemainingSymbolsCounter;
        ctrl.isFieldInvalid = isFieldInvalid;
        ctrl.isCounterVisible = isCounterVisible;
        ctrl.focusInput = focusInput;
        ctrl.keyDown = keyDown;
        ctrl.unfocusInput = unfocusInput;
        ctrl.updateInputValue = updateInputValue;
        ctrl.clearInputField = clearInputField;

        //
        // Hook methods
        //

        /**
         * Initialization method
         */
        function onInit() {
            if (!lodash.isNil(ctrl.disabled)) {
                ctrl.disableField = ctrl.disabled;
            }

            ctrl.inputModelOptions = lodash.defaultsDeep(ctrl.inputModelOptions || {}, defaultInputModelOptions);

            ctrl.inputFocused = ctrl.isFocused;
            ctrl.spellcheck = ctrl.spellcheck || 'true';

            ctrl.data = angular.copy(lodash.defaultTo(ctrl.inputValue, ''));
            ctrl.startValue = angular.copy(ctrl.inputValue);
        }

        /**
         * Method called after initialization
         */
        function postLink() {
            if (ctrl.isFocused) {

                // check is this input field is in dialog
                if (angular.isDefined($element.closest('.ngdialog')[0])) {
                    angular.element($window).on('animationend', function (event) {

                        if (event.originalEvent.animationName === 'ngdialog-fadein' && event.target.className === 'ngdialog-content') {
                            $timeout(function () {
                                $element.find('.field')[0].focus();
                                angular.element($window).off('animationend');
                            }, 300);
                        }
                    });
                } else {
                    $timeout(function () {
                        $element.find('.field')[0].focus();
                    });
                }
            }
        }

        /**
         * Destructor
         */
        function onDestroy() {
            angular.element($window).off('animationend');
        }

        /**
         * onChange hook
         * @param {Object} changes
         */
        function onChanges(changes) {
            if (angular.isDefined(changes.inputValue)) {
                if (!changes.inputValue.isFirstChange()) {
                    ctrl.data = angular.copy(changes.inputValue.currentValue);
                    ctrl.startValue = angular.copy(ctrl.inputValue);
                }
            }

            if (angular.isDefined(changes.isFocused)) {
                if (!changes.isFocused.isFirstChange()) {
                    $timeout(function () {
                        $element.find('.field')[0].focus();
                    });
                }
            }
        }

        //
        // Public methods
        //

        /**
         * Get counter of the remaining symbols for the field
         * @returns {number}
         */
        function getRemainingSymbolsCounter() {
            if (ctrl.formObject) {
                var maxLength = parseInt(ctrl.validationMaxLength);
                var inputViewValue = ctrl.formObject[ctrl.inputName].$viewValue;

                return (maxLength >= 0 && inputViewValue) ? (maxLength - inputViewValue.length).toString() : null;
            }
        }

        /**
         * Check whether the field is invalid.
         * Do not validate field if onlyValidCharacters parameter was passed.
         * @returns {boolean}
         */
        function isFieldInvalid() {
            return !ctrl.onlyValidCharacters ? FormValidationService.isShowFieldInvalidState(ctrl.formObject, ctrl.inputName) : false;
        }

        /**
         * Check whether the counter should be visible
         * @returns {boolean}
         */
        function isCounterVisible() {
            return lodash.isNil(ctrl.hideCounter) || ctrl.hideCounter === 'false' ? true : false;
        }

        /**
         * Method to make input unfocused
         */
        function focusInput() {
            ctrl.inputFocused = true;
            if (angular.isFunction(ctrl.itemFocusCallback)) {
                ctrl.itemFocusCallback();
            }
        }

        /**
         * Method which have been called from 'keyDown' event
         * @param {Object} event - native event object
         */
        function keyDown(event) {
            if (angular.isDefined(ctrl.enterCallback) && event.keyCode === EventHelperService.ENTER) {
                $timeout(ctrl.enterCallback);
            }
        }

        /**
         * Method to make input unfocused
         */
        function unfocusInput() {
            ctrl.inputFocused = false;

            // If 'data revert' option is enabled - set or revert outer model value
            setOrRevertInputValue();
        }

        /**
         * Updates outer model value on inner model value change
         * Used for `ng-change` directive
         */
        function updateInputValue() {
            if (angular.isDefined(ctrl.data)) {
                ctrl.inputValue = angular.isString(ctrl.data) ? ctrl.data.trim() : ctrl.data;
            }

            if (angular.isDefined(ctrl.updateDataCallback)) {
                ctrl.updateDataCallback({newData: ctrl.inputValue, field: angular.isDefined(ctrl.updateDataField) ? ctrl.updateDataField : ctrl.inputName});
            }
        }

        /**
         * Clear search input field
         */
        function clearInputField() {
            ctrl.data = '';
            updateInputValue();
        }

        //
        // Private methods
        //

        /**
         * Sets or reverts outer model value
         */
        function setOrRevertInputValue() {
            $timeout(function () {
                if (ctrl.isDataRevert === 'true') {

                    // If input is invalid - inner model value is set to undefined by Angular
                    if (angular.isDefined(ctrl.data) && ctrl.startValue !== Number(ctrl.data)) {
                        ctrl.inputValue = angular.isString(ctrl.data) ? ctrl.data.trim() : ctrl.data;
                        if (angular.isFunction(ctrl.itemBlurCallback)) {
                            ctrl.itemBlurCallback({inputValue: ctrl.inputValue});
                        }
                        ctrl.startValue = Number(ctrl.data);
                    } else {

                        // Revert input value; Outer model value just does not change
                        ctrl.data = ctrl.inputValue;
                        if (angular.isFunction(ctrl.onBlur)) {
                            ctrl.onBlur();
                        }
                    }
                }
            });
        }
    }
}());

(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('igzNumberInput', {
            bindings: {
                currentValue: '<',
                currentValueUnit: '<',
                formObject: '<',
                onChange: '<?',
                isDisabled: '<?',
                validationValue: '<',
                validationValueUnit: '<',
                disableZeroValue: '<?',
                updateNumberInputCallback: '&?',
                defaultValue: '@',
                inputName: '@',
                isFocused: '@',
                maxValue: '<',
                minValue: '<',
                placeholder: '@',
                precision: '@decimalNumber',
                prefixUnit: '@',
                suffixUnit: '@',
                valueStep: '@',
                updateNumberInputField: '@?'
            },
            templateUrl: 'shared-dashboard-controls/components/container-wizard-number-input/container-wizard-number-input.tpl.html',
            controller: IgzNumberInputController
        });

    function IgzNumberInputController($timeout, $element, lodash, FormValidationService) {
        var ctrl = this;

        ctrl.numberInputValid = true;
        ctrl.numberInputChanged = false;
        ctrl.precision = Number(ctrl.precision) || 0;
        ctrl.placeholder = ctrl.placeholder || '';

        ctrl.$onInit = onInit;
        ctrl.$postLink = postLink;

        ctrl.isShowFieldInvalidState = FormValidationService.isShowFieldInvalidState;

        ctrl.checkInvalidation = checkInvalidation;
        ctrl.decreaseValue = decreaseValue;
        ctrl.increaseValue = increaseValue;
        ctrl.isShownUnit = isShownUnit;
        ctrl.onChangeInput = onChangeInput;
        ctrl.onBlurInput = onBlurInput;

        //
        // Hook methods
        //

        /**
         * Initialization method
         */
        function onInit() {
            ctrl.defaultValue = Number.isNaN(Number(ctrl.defaultValue)) ? null : Number(ctrl.defaultValue);
            resizeInput();
        }

        /**
         * Post linking method
         */
        function postLink() {
            ctrl.inputFocused = ctrl.isFocused === 'true';

            if (ctrl.isFocused === 'true') {
                $element.find('.field')[0].focus();
            }
        }

        //
        // Public methods
        //

        /**
         * Checks if the input value is invalid
         * @returns {boolean}
         */
        function checkInvalidation() {
            return ctrl.isShowFieldInvalidState(ctrl.formObject, ctrl.inputName);
        }

        /**
         * Method subtracts value from current value in input or sets current value to 0 it is below 0
         */
        function decreaseValue() {
            ctrl.currentValue = Math.max(Number(ctrl.currentValue) - Number(ctrl.valueStep), 0).toFixed(ctrl.precision);

            if (angular.isDefined(ctrl.formObject)) {
                ctrl.formObject[ctrl.inputName].$setViewValue(ctrl.currentValue.toString());
                ctrl.formObject[ctrl.inputName].$render();
            }

            // if value becomes zero - clear the input field
            if (ctrl.currentValue === 0 && ctrl.disableZeroValue) {
                ctrl.currentValue = null;
            }

            onCurrentValueChange();
        }

        /**
         * Method adds value to current value in input
         */
        function increaseValue() {
            ctrl.currentValue = (Number(ctrl.currentValue) + Number(ctrl.valueStep)).toFixed(ctrl.precision);

            if (angular.isDefined(ctrl.formObject)) {
                ctrl.formObject[ctrl.inputName].$setViewValue(ctrl.currentValue.toString());
                ctrl.formObject[ctrl.inputName].$render();
            }

            resizeInput();
            onCurrentValueChange();
        }

        /**
         * Method checks if passed value is defined
         * @param {string} [unitValue] - passed string unit value
         * @returns {boolean} returns true if defined
         */
        function isShownUnit(unitValue) {
            return angular.isDefined(unitValue);
        }

        /**
         * Handles on change event
         */
        function onChangeInput() {
            ctrl.numberInputChanged = true;
            resizeInput();
        }

        /**
         * Handles on blur event
         */
        function onBlurInput() {
            ctrl.inputFocused = false;
            onCurrentValueChange();
        }

        //
        // Private methods
        //

        /**
         * Handles any changes of current value
         */
        function onCurrentValueChange() {
            validateCurrentValue();
            $timeout(function () {
                lodash.get(ctrl, 'onChange', angular.noop)(ctrl.checkInvalidation());
                resizeInput();
            }, 0);
        }

        /**
         * Resizes number input width
         */
        function resizeInput() {
            var numberInput = $element.find('input')[0];
            if (!lodash.isNil(numberInput)) {
                numberInput.size = !lodash.isEmpty(ctrl.currentValue) || lodash.isNumber(ctrl.currentValue) ?
                                   ctrl.currentValue.toString().length : !lodash.isEmpty(ctrl.placeholder) ?
                                   ctrl.placeholder.length : 1;
            }
        }

        /**
         * Resets the input to default value if it is invalid
         */
        function validateCurrentValue() {
            ctrl.numberInputValid = ctrl.checkInvalidation();
            if (ctrl.numberInputValid) {
                ctrl.currentValue = Number(ctrl.defaultValue);
            }

            if (angular.isFunction(ctrl.updateNumberInputCallback)) {
                ctrl.updateNumberInputCallback({newData: ctrl.currentValue, field: angular.isDefined(ctrl.updateNumberInputField) ? ctrl.updateNumberInputField : ctrl.inputName});
            }
        }
    }
}());

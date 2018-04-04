describe('igzNumberInput component:', function () {
    var $componentController;
    var $rootScope;
    var ctrl;
    var stepSecondServiceObjectives;

    beforeEach(function () {
        module('iguazio.app');

        inject(function (_$componentController_, _$rootScope_) {
            $componentController = _$componentController_;
            $rootScope = _$rootScope_;
        });

        stepSecondServiceObjectives = {
            latencyTargetTimeLimit: 150
        };

        var unitValue = 'ms';
        var secondStepForm = {
            latencyTarget: {
                $setViewValue: angular.noop,
                $render: angular.noop
            }
        };
        var bindings = {
            formObject: secondStepForm,
            inputName: 'latencyTarget',
            isFocused: true,
            currentValue: stepSecondServiceObjectives.latencyTargetTimeLimit,
            placeholder: 'None',
            decimalNumber: 0,
            minValue: 0,
            valueStep: 10,
            suffixUnit: unitValue
        };
        var element = angular.element('<igz-number-input></igz-number-input>');

        ctrl = $componentController('igzNumberInput', {$element: element}, bindings);
    });

    afterEach(function () {
        $componentController = null;
        $rootScope = null;
        ctrl = null;
        stepSecondServiceObjectives = null;
    });

    describe('initial state: ', function () {
        it('should be rendered with correct data', function () {
            expect(Number(ctrl.precision)).toBe(0);
            expect(ctrl.placeholder).toBe('None');
        });
    });

    describe('increaseValue(): ', function () {
        it('should increase current value', function () {
            expect(ctrl.currentValue).toEqual(stepSecondServiceObjectives.latencyTargetTimeLimit);
            ctrl.increaseValue();
            expect(Number(ctrl.currentValue)).toEqual(160);
            ctrl.increaseValue();
            ctrl.increaseValue();
            ctrl.increaseValue();
            expect(Number(ctrl.currentValue)).toEqual(190);
        });
    });

    describe('decreaseValue(): ', function () {
        it('should decrease current value', function () {
            expect(ctrl.currentValue).toEqual(stepSecondServiceObjectives.latencyTargetTimeLimit);
            ctrl.decreaseValue();
            expect(Number(ctrl.currentValue)).toEqual(140);
        });

        it('should set current value to "default value" when it is bellow 0', function () {
            ctrl.defaultValue = 0;
            spyOn(ctrl, 'isShowFieldInvalidState').and.returnValue(true);
            expect(ctrl.currentValue).toEqual(stepSecondServiceObjectives.latencyTargetTimeLimit);
            ctrl.currentValue = 0;
            ctrl.decreaseValue();
            expect(ctrl.currentValue).toEqual(ctrl.defaultValue);
            ctrl.currentValue = -50;
            ctrl.decreaseValue();
            expect(ctrl.currentValue).toEqual(ctrl.defaultValue);
        });
    });

    describe('isShownUnit(): ', function () {
        it('should return true', function () {
            var componentScopeUnitValue;
            componentScopeUnitValue = ctrl.isShownUnit(ctrl.suffixUnit);
            expect(componentScopeUnitValue).toBeTruthy();
        });

        it('should return false', function () {
            var componentScopeUnitValue;
            ctrl.suffixUnit = undefined;
            componentScopeUnitValue = ctrl.isShownUnit(ctrl.suffixUnit);
            expect(componentScopeUnitValue).toBeFalsy();
        });
    });

    describe('checkInvalidation(): ', function () {
        beforeEach(function () {
            ctrl.formObject = {};
            ctrl.inputName = 'input_name';
        });

        it('should call isShowFieldInvalidState method', function () {
            spyOn(ctrl, 'isShowFieldInvalidState').and.returnValue(true);
            ctrl.checkInvalidation();

            expect(ctrl.isShowFieldInvalidState).toHaveBeenCalled();
        });
    });
});
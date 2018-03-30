describe('igzValidatingInputField component:', function () {
    var $componentController;
    var $timeout;
    var ctrl;
    var defaultInputModelOptions;

    beforeEach(function () {
        module('iguazio.app');

        inject(function (_$componentController_, _$timeout_) {
            $componentController = _$componentController_;
            $timeout = _$timeout_;
        });

        defaultInputModelOptions = {
            updateOn: 'default blur',
            debounce: {
                'default': 1000,
                'blur': 0
            },
            allowInvalid: true
        };

        var formObject = {
            attributeName: {
                $viewValue: 'some value'
            }
        };
        var bindings = {
            formObject: formObject,
            inputName: 'attributeName',
            inputValue: 'some input value',
            itemBlurCallback: angular.noop,
            itemFocusCallback: angular.noop,
            isDataRevert: 'true',
            isFocused: 'true',
            updateDataCallback: angular.noop,
            updateDataField: angular.noop,
            validationMaxLength: '15'
        };
        var element = '<igz-validating-input-field></igz-validating-input-field>';

        ctrl = $componentController('igzValidatingInputField', {$element: element}, bindings);
        ctrl.$onInit();
    });

    afterEach(function () {
        $componentController = null;
        $timeout = null;
        ctrl = null;
        defaultInputModelOptions = null;
    });

    describe('initial state:', function () {
        it('should be rendered with correct data', function () {
            expect(ctrl.inputModelOptions).toEqual(defaultInputModelOptions);
            expect(ctrl.inputFocused).toBeTruthy();
            expect(ctrl.spellcheck).toBeTruthy();
            expect(ctrl.data).toBe(ctrl.inputValue)
        });
    });

    describe('$onChanges():', function () {
        it('should set new value to ctrl.data', function () {
            var changes = {
                inputValue: {
                    currentValue: 'some new value',
                    isFirstChange: function () {
                        return false;
                    }
                }
            };
            ctrl.$onChanges(changes);

            expect(ctrl.data).toEqual(changes.inputValue.currentValue);
        })
    });

    describe('getRemainingSymbolsCounter():', function () {
        it('should return difference between $viewValue length and validationMaxLength', function () {
            expect(ctrl.getRemainingSymbolsCounter()).toBe('5')
        })
    });

    describe('focusInput():', function () {
        it('should call ctrl.itemFocusCallback', function () {
            var spy = spyOn(ctrl, 'itemFocusCallback');
            ctrl.focusInput();

            expect(spy).toHaveBeenCalled();
        });
    });

    describe('unfocusInput():', function () {
        it('should call ctrl.itemBlurCallback with ctrl.inputValue', function () {
            ctrl.data = 'new value';
            var spy = spyOn(ctrl, 'itemBlurCallback');
            ctrl.unfocusInput();
            $timeout(function () {
                expect(spy).toHaveBeenCalledWith({inputValue: ctrl.inputValue});
            });
            $timeout.flush();
        });
    });

    describe('updateInputValue():', function () {
        it('should set ctrl.inputValue if ctrl.data is defined', function () {
            ctrl.updateInputValue();

            expect(ctrl.inputValue).toBe(ctrl.data)
        });

        it('should call ctrl.updateDataCallback with ctrl.inputValue, and ctrl.updateDataField if ctrl.updateDataField' +
            'is defined', function () {
            var spy = spyOn(ctrl, 'updateDataCallback');
            ctrl.updateInputValue();

            expect(spy).toHaveBeenCalledWith({newData: ctrl.inputValue, field: ctrl.updateDataField})
        });
    });

    describe('clearInputField()', function () {
        it('should empty search field after call clearInputField()', function () {
            ctrl.data = 'new';
            ctrl.clearInputField();
            expect(ctrl.data).toEqual('');
        });
    })
});

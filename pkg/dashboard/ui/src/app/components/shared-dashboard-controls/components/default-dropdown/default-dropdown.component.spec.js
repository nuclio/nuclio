describe('igzDefaultDropdown component:', function () {
    var $componentController;
    var $event;
    var $rootScope;
    var $timeout;
    var lodash;
    var FormValidationService;
    var PriorityDropdownService;
    var ctrl;
    var isRequired;
    var types;

    beforeEach(function () {
        module('iguazio.app');

        inject(function (_$componentController_, _$rootScope_, _$timeout_, _lodash_, _FormValidationService_, _PriorityDropdownService_) {
            $componentController = _$componentController_;
            $rootScope = _$rootScope_;
            $timeout = _$timeout_;
            lodash = _lodash_;
            FormValidationService = _FormValidationService_;
            PriorityDropdownService = _PriorityDropdownService_;
        });

        $event = {
            currentTarget: {
                getBoundingClientRect: function () {
                    return {
                        height: 500,
                        top: 500
                    }
                }
            }
        };
        isRequired = true;
        types = [
            {
                id: 'memory',
                name: 'Memory'
            },
            {
                id: 'performance_read_write',
                name: 'Performance - Read/Write'
            },
            {
                id: 'performance_read_mostly',
                name: 'Performance - Read mostly'
            },
            {
                id: 'capacity',
                name: 'Capacity'
            },
            {
                id: 'hybrid',
                name: 'Hybrid'
            },
            {
                id: 'secondary',
                name: 'Secondary'
            }
        ];

        var selectedItem = types[1].id;
        var bindings = {
            isRequired: isRequired,
            selectPropertyOnly: 'id',
            valuesArray: types,
            selectedItem: selectedItem,
            itemSelectCallback: angular.noop,
            placeholder: 'Select...'
        };
        var element = angular.element('<igz-default-dropdown><div class="default-dropdown-field"></div></igz-default-dropdown>');

        ctrl = $componentController('igzDefaultDropdown', {$element: element, $transclude: angular.noop}, bindings);
    });

    afterEach(function () {
        $componentController = null;
        $event = null;
        $rootScope = null;
        $timeout = null;
        lodash = null;
        FormValidationService = null;
        PriorityDropdownService = null;
        ctrl = null;
    });

    describe('onInit(): ', function () {
        it('should call initialization methods when dropdown type is a \'priority\'', function () {
            var valuesArray = PriorityDropdownService.getPrioritiesArray();
            lodash.forEach(valuesArray, function (value) {
                lodash.defaults(value, {visible: true});
            });
            ctrl.dropdownType = 'priority';

            ctrl.$onInit();

            expect(ctrl.valuesArray).toEqual(valuesArray);
            expect(ctrl.typedValue).toBe('');
            expect(ctrl.placeholder).toBe('Select...');
        });

        it('should call initialization methods when dropdown type is not defined', function () {
            var valuesArray = angular.copy(ctrl.valuesArray);
            lodash.forEach(valuesArray, function (value) {
                lodash.defaults(value, {visible: true});
            });
            ctrl.dropdownType = null;

            ctrl.$onInit();

            expect(ctrl.typedValue).toBe('Performance - Read/Write');
            expect(ctrl.placeholder).toBe('Select...');
            expect(ctrl.valuesArray).toEqual(valuesArray);
        });
    });

    describe('checkIsRequired(): ', function () {
        it('should return required flag as it was passes throw scope', function () {
            expect(ctrl.checkIsRequired()).toBe(isRequired);
        });

        it('should return correct required flag', function () {
            ctrl.isRequired = false;
            expect(ctrl.checkIsRequired()).toBeFalsy();
        });
    });

    describe('getValuesArray():', function () {
        it('should return correct array of available values', function () {
            expect(ctrl.getValuesArray()).toBe(types);
            ctrl.valuesArray = null;
            expect(ctrl.getValuesArray()).toBeNull();
        });
    });

    describe('getTooltip():', function () {
        it('should return correct tooltip of selected value', function () {
            var item = {
                tooltip: 'Item\'s tooltip'
            };

            expect(ctrl.getTooltip(item)).toEqual(item.tooltip);
        });
    });

    describe('isItemSelected():', function () {
        it('should determine whether current item selected when "selectPropertyOnly" is defined', function () {
            var result;

            ctrl.selectedItem = ctrl.valuesArray[1].id;
            result = ctrl.isItemSelected(ctrl.valuesArray[2]);

            expect(result).toBeFalsy();
            result = ctrl.isItemSelected(ctrl.valuesArray[1]);
            expect(result).toBeTruthy();
        });

        it('should determine whether current item selected when "selectPropertyOnly" is undefined', function () {
            var result;

            delete ctrl.selectPropertyOnly;
            ctrl.selectedItem = ctrl.valuesArray[1];
            result = ctrl.isItemSelected(ctrl.valuesArray[2]);
            expect(result).toBeFalsy();
            result = ctrl.isItemSelected(ctrl.valuesArray[1]);
            expect(result).toBeTruthy();
        });
    });

    describe('isPlaceholderClass():', function () {
        it('should return correct result when "selectPropertyOnly" is defined', function () {
            var result;

            result = ctrl.isPlaceholderClass();
            expect(result).toBeFalsy();
            ctrl.selectedItem = null;
            result = ctrl.isPlaceholderClass();
            expect(result).toBeTruthy();
        });

        it('should return correct result when "selectPropertyOnly" is undefined', function () {
            var result;

            delete ctrl.selectPropertyOnly;
            ctrl.selectedItem = ctrl.valuesArray[4];
            result = ctrl.isPlaceholderClass();

            expect(result).toBeFalsy();
            ctrl.selectedItem.id = null;
            result = ctrl.isPlaceholderClass();
            expect(result).toBeTruthy();
        });
    });

    describe('isShowDropdownError():', function () {
        it('should call service method when "isRequired" property equals true', function () {
            spyOn(FormValidationService, 'isShowFieldInvalidState').and.stub();
            ctrl.isRequired = true;
            ctrl.isShowDropdownError();

            expect(FormValidationService.isShowFieldInvalidState).toHaveBeenCalled();
        });

        it('should not call service method when "isRequired" property equals false', function () {
            spyOn(FormValidationService, 'isShowFieldInvalidState').and.stub();
            ctrl.isRequired = false;
            ctrl.isShowDropdownError();

            expect(FormValidationService.isShowFieldInvalidState).not.toHaveBeenCalled();
        });
    });

    describe('onChangeTypingInput():', function () {
        it('should change selected item after typing value if this value present in valuesArray', function () {
            spyOn(ctrl, 'itemSelectCallback').and.callThrough();

            ctrl.selectedItem = ctrl.valuesArray[0];
            ctrl.typedValue = 'Performance - Read/Write';
            ctrl.itemSelectField = 'attr';
            ctrl.onChangeTypingInput();
            $timeout.flush();

            var expectedResults = {
                item: ctrl.valuesArray[1],
                isItemChanged: true,
                field: ctrl.itemSelectField
            };

            expect(ctrl.itemSelectCallback).toHaveBeenCalledWith(expectedResults);
        });

        it('should change selected item after typing value if this value isn\'t present in valuesArray (add new item) when nameKey is not defined', function () {
            spyOn(ctrl, 'itemSelectCallback').and.callThrough();

            ctrl.nameKey = null;
            ctrl.typedValue = 'Performance1234';
            ctrl.itemSelectField = 'attr';
            ctrl.onChangeTypingInput();
            $timeout.flush();

            var expectedResults = {
                item: {
                    id: ctrl.typedValue,
                    name: ctrl.typedValue,
                    visible: true
                },
                isItemChanged: true,
                field: ctrl.itemSelectField
            };

            expect(ctrl.itemSelectCallback).toHaveBeenCalledWith(expectedResults);
        });

        it('should change selected item after typing value if this value isn\'t present in valuesArray (add new item) when nameKey is defined', function () {
            spyOn(ctrl, 'itemSelectCallback').and.callThrough();

            ctrl.nameKey = 'attr.name';
            ctrl.typedValue = 'Performance1234';
            ctrl.itemSelectField = 'attr';
            ctrl.onChangeTypingInput();
            $timeout.flush();

            var expectedResults = {
                item: {
                    id: ctrl.typedValue,
                    attr: {
                        name: ctrl.typedValue
                    },
                    visible: true
                },
                isItemChanged: true,
                field: ctrl.itemSelectField
            };

            expect(ctrl.itemSelectCallback).toHaveBeenCalledWith(expectedResults);
        });
    });

    describe('selectItem():', function () {
        it('should set correct data when item selects when "selectPropertyOnly" is defined', function () {
            var item = ctrl.valuesArray[4];
            item['description'] = 'description for selected item';

            ctrl.selectItem(item);

            expect(ctrl.selectedItem).toBe(item.id);
            expect(ctrl.selectedItemDescription).toBe(item.description);
            expect(ctrl.typedValue).toBe(item.name);
        });

        it('should set correct data when item selects when "selectPropertyOnly" is undefined', function () {
            var item = ctrl.valuesArray[4];

            delete ctrl.selectPropertyOnly;
            ctrl.selectedItem = ctrl.valuesArray[0];
            ctrl.selectItem(item);

            expect(ctrl.selectedItem).toBe(item);
            expect(ctrl.typedValue).toBe(item.name);
        });

        it('should set property "isDropdownContainerShown" to false', function () {
            ctrl.isDropdownContainerShown = true;
            ctrl.selectItem(ctrl.valuesArray[0]);

            expect(ctrl.isDropdownContainerShown).toBeFalsy();
        });
    });

    describe('showSelectedItem():', function () {
        it('should set default data into "selectedItem" if the last one equals null when "selectPropertyOnly" is defined', function () {
            ctrl.selectedItem = null;
            ctrl.showSelectedItem();

            expect(ctrl.selectedItem).toBeNull();
        });

        it('should set default data into "selectedItem" if the last one equals null when "selectPropertyOnly" is undefined', function () {
            ctrl.selectedItem = null;
            delete ctrl.selectPropertyOnly;
            ctrl.showSelectedItem();

            expect(ctrl.selectedItem.id).toBeNull();
            expect(ctrl.selectedItem.name).toBeNull();
        });

        it('should set empty string to "hiddenInputValue" property', function () {
            ctrl.selectedItem = null;
            ctrl.showSelectedItem();

            expect(ctrl.hiddenInputValue).toBe('');
        });

        it('should return correct "selectedItem" property when "selectPropertyOnly" is defined', function () {
            var selectedItem;
            var item = ctrl.valuesArray[0];

            item['description'] = 'description for selected item';
            ctrl.selectedItem = item.id;
            selectedItem = ctrl.showSelectedItem();

            expect(selectedItem.name).toBe(item.name);
            expect(selectedItem.description).toBe(item.description);
        });

        it('should return correct "selectedItem" property when "selectPropertyOnly" is undefined', function () {
            var selectedItem;

            ctrl.selectedItem = ctrl.valuesArray[5];
            ctrl.selectPropertyOnly = null;
            selectedItem = ctrl.showSelectedItem();

            expect(selectedItem.name).toBe(ctrl.placeholder);
            expect(selectedItem.description).toBeNull();
        });
    });

    describe('toggleDropdown():', function () {
        it('should set correct value into property', function () {
            ctrl.isDropdownContainerShown = false;
            ctrl.toggleDropdown($event);

            expect(ctrl.isDropdownContainerShown).toBeTruthy();
            ctrl.toggleDropdown($event);
            expect(ctrl.isDropdownContainerShown).toBeFalsy();
        });
    });

    describe('onDropDownKeydown():', function () {
        it('dropdown should not be shown', function () {
            var event = {
                target: {blur: angular.noop},
                keyCode: 9,
                stopPropagation: angular.noop
            };

            ctrl.onDropDownKeydown(event);

            expect(ctrl.isDropdownContainerShown).toBeFalsy();
        });
    });

    describe('onItemKeydown():', function () {
        it('should call selectItem() method', function () {
            var item = ctrl.valuesArray[4];
            item['description'] = 'description for selected item';
            var event = {
                preventDefault: angular.noop,
                keyCode: 13,
                stopPropagation: angular.noop
            };
            spyOn(ctrl, 'selectItem');
            ctrl.onItemKeydown(event, item);

            expect(ctrl.selectItem).toHaveBeenCalled();
        });
    });
});
describe('igzActionCheckbox component:', function () {
    var $componentController;
    var $rootScope;
    var ctrl;

    beforeEach(function () {
        module('iguazio.app');

        inject(function (_$componentController_, _$rootScope_) {
            $componentController = _$componentController_;
            $rootScope = _$rootScope_;
        });

        var bindings = {
            item : {
                id: 1,
                ui: {
                    checked: false
                }
            },
            onClickCallback: angular.noop
        };

        ctrl = $componentController('igzActionCheckbox', null, bindings);
    });

    afterEach(function () {
        $componentController = null;
        $rootScope = null;
        ctrl = null;
    });

    describe('onCheck(): ', function () {
        it ('should be inited and unchecked by default', function () {
            expect(ctrl.item.ui.checked).toBe(false);
        });

        it ('Element should be checked and broadcast should be sent:', function () {
            spyOn($rootScope, '$broadcast');

            var event = {
                stopPropagation: function () {},
                preventDefault: function () {}
            };
            ctrl.onCheck(event);

            expect(ctrl.item.ui.checked).toBe(true);
            expect($rootScope.$broadcast).toHaveBeenCalledWith('action-checkbox_item-checked', {checked: true});
        });
    });
});

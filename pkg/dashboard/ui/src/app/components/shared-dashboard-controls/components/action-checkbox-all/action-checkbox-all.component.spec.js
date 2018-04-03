describe('igzActionCheckboxAll component:', function () {
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
            itemsCount: 10,
            checkedItemsCount: 4
        };

        ctrl = $componentController('igzActionCheckboxAll', null, bindings);
    });

    afterEach(function () {
        $componentController = null;
        $rootScope = null;
        ctrl = null;
    });

    describe('onCheckAll(): ', function () {
        it('Check all broadcast should be sent:', function () {
            spyOn($rootScope, '$broadcast');

            ctrl.onCheckAll();
            expect(ctrl.checkedItemsCount).toBe(10);
            expect($rootScope.$broadcast).toHaveBeenCalledWith('action-checkbox-all_check-all', {
                checked: true,
                checkedCount: 10
            });

            ctrl.onCheckAll();
            expect(ctrl.checkedItemsCount).toBe(0);
            expect($rootScope.$broadcast).toHaveBeenCalledWith('action-checkbox-all_check-all', {
                checked: false,
                checkedCount: 0
            });
        });
    });

});
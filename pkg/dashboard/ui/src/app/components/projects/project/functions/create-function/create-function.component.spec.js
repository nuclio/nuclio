describe('nclCreateFunction Component:', function () {
    var $componentController;
    var ctrl;

    beforeEach(function () {
        module('iguazio.app');

        inject(function (_$componentController_) {
            $componentController = _$componentController_;
        });

        ctrl = $componentController('nclCreateFunction', null);
    });

    afterEach(function () {
        $componentController = null;
        ctrl = null;
    });

    describe('isTypeSelected():', function () {
        it('should return true if "functionType" is equal to "selectedFunctionType"', function () {
            expect(ctrl.isTypeSelected('from_template')).toBeFalsy();
        });

        it('should return false if "functionType" is not equal to "selectedFunctionType"', function () {
            expect(ctrl.isTypeSelected('from_scratch')).toBeTruthy();
        });
    });
});
describe('nclProjectsSortDropdown component:', function () {
    var $componentController;
    var ctrl;

    beforeEach(function () {
        module('iguazio.app');

        inject(function (_$componentController_) {
            $componentController = _$componentController_;
        });

        ctrl = $componentController('nclProjectsSortDropdown');
    });

    afterEach(function () {
        $componentController = null;
        ctrl = null;
    });

    describe('$onInit():', function () {

        // todo
    });
});
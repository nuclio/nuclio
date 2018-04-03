describe('igzInfoPageActionsBar component:', function () {
    var $componentController;
    var $rootScope;
    var ctrl;

    beforeEach(function () {
        module('iguazio.app');

        inject(function (_$componentController_, _$rootScope_) {
            $componentController = _$componentController_;
            $rootScope = _$rootScope_
        });

        var bindings = {
            watchId: 1
        };

        ctrl = $componentController('igzInfoPageActionsBar', null, bindings);
        ctrl.$onInit();

        $rootScope.$broadcast('info-page-upper-pane_toggle-start-1', true);
        $rootScope.$broadcast('info-page-filters_toggle-start-1', true);
        $rootScope.$broadcast('info-page-pane_toggle-start-1', true);
    });

    afterEach(function () {
        $componentController = null;
        $rootScope = null;
        ctrl = null;
    });

    describe('initial state:', function () {
        it('should set value from broadcast to ctrl.isUpperPaneShowed', function () {
            expect(ctrl.isUpperPaneShowed).toBeTruthy();
        });

        it('should set value from broadcast to ctrl.isFiltersShowed', function () {
            expect(ctrl.isFiltersShowed).toBeTruthy();
        });

        it('should set value from broadcast to ctrl.isInfoPaneShowed', function () {
            expect(ctrl.isInfoPaneShowed).toBeTruthy();
        });
    });
});

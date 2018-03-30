describe('igzInfoPageFilters component:', function () {
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
            changeStateCallback: angular.noop,
            resetFilters: angular.noop,
            applyFilters: angular.noop,
            watchId: 1
        };
        var element = '<igz-info-page-filters></igz-info-page-filters>';

        ctrl = $componentController('igzInfoPageFilters', {$element: element}, bindings);
        ctrl.$onInit();

        $rootScope.$broadcast('info-page-upper-pane_toggle-start-1', true);
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

        it('should call ctrl.changeStateCallback', function () {
            var spy = spyOn(ctrl, 'changeStateCallback');
            $rootScope.$broadcast('info-page-pane_toggle-start-1', true);

            expect(spy).toHaveBeenCalled();
        })
    });

    describe('onApplyFilters():', function () {
        it('should call $broadcast and ctrl.applyFilters', function () {
            var broadcastSpy = spyOn($rootScope, '$broadcast');
            var applyFiltersSpy = spyOn(ctrl, 'applyFilters');
            ctrl.onApplyFilters();

            expect(broadcastSpy).toHaveBeenCalled();
            expect(applyFiltersSpy).toHaveBeenCalled();
        })
    });

    describe('onResetFilters():', function () {
        it('should call $broadcast and ctrl.resetFilters', function () {
            var broadcastSpy = spyOn($rootScope, '$broadcast');
            var resetFiltersSpy = spyOn(ctrl, 'resetFilters');
            ctrl.onResetFilters();

            expect(broadcastSpy).toHaveBeenCalled();
            expect(resetFiltersSpy).toHaveBeenCalled();
        })
    });

    describe('isShowFooterButtons():', function () {
        it('should return true if ctrl.resetFilters or ctrl.applyFilters is functions', function () {
            expect(ctrl.isShowFooterButtons()).toBeTruthy();
        })
    })
});

describe('igzSplashScreen component:', function () {
    var $componentController;
    var $rootScope;
    var $state;
    var ctrl;

    beforeEach(function () {
        module('iguazio.app');

        inject(function (_$componentController_, _$rootScope_, _$state_) {
            $componentController = _$componentController_;
            $rootScope = _$rootScope_;
            $state = _$state_;
        });

        ctrl = $componentController('igzSplashScreen', null);

        ctrl.$onInit();
    });

    afterEach(function () {
        $componentController = null;
        $rootScope = null;
        $state = null;
        ctrl = null;
    });

    describe('refreshPage()', function () {
        it('should send broadcast, and set isLoading to true, isFailedBrowseService to false', function () {
            var reloadSpy = spyOn($state, 'reload');

            ctrl.isLoading = false;
            ctrl.isAlertShowing = true;

            ctrl.refreshPage();

            expect(reloadSpy).toHaveBeenCalled();
            expect(ctrl.isLoading).toBeTruthy();
            expect(ctrl.isAlertShowing).toBeFalsy();
        });
    });
});

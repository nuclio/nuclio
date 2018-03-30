describe('igzActionsPanes component:', function () {
    var $componentController;
    var $rootScope;
    var ConfigService;
    var ctrl;
    var closeInfoPane;
    var infoPaneToggleMethod;

    beforeEach(function () {
        module('iguazio.app');

        inject(function (_$componentController_, _$rootScope_, _ConfigService_) {
            $componentController = _$componentController_;
            $rootScope = _$rootScope_;
            ConfigService = _ConfigService_;
        });

        closeInfoPane = function () {
            return 'close-info-pane';
        };

        infoPaneToggleMethod = function () {
            return 'close-info-pane';
        };

        var bindings = {
            showFilterIcon: 'true',
            closeInfoPane: closeInfoPane,
            infoPaneToggleMethod: infoPaneToggleMethod,
            filtersToggleMethod: angular.noop
        };

        ctrl = $componentController('igzActionsPanes', null, bindings);
    });

    afterEach(function () {
        $componentController = null;
        $rootScope = null;
        ConfigService = null;
        ctrl = null;
        closeInfoPane = null;
        infoPaneToggleMethod = null;
    });

    describe('onInit(): ', function () {
        it('should set ctrl.callToggleMethod to ctrl.closeInfoPane if it exists', function () {
            ctrl.$onInit();

            expect(ctrl.callToggleMethod).toEqual(closeInfoPane);
        });

        it('should set ctrl.callToggleMethod to ctrl.infoPaneToggleMethod if it does\'t exist', function () {
            ctrl.closeInfoPane = 'not a function';

            ctrl.$onInit();

            expect(ctrl.callToggleMethod).toEqual(infoPaneToggleMethod);
        });
    });

    describe('ctrl.isShowFilterActionIcon(): ', function () {
        it('should return true if filter toggle method exists and if ctrl.showFilterIcon is true', function () {
            ctrl.isShowFilterActionIcon();

            expect(ctrl.isShowFilterActionIcon()).toBeTruthy();
        });

        it('should return true if filter toggle method exists and if ctrl.showFilterIcon is false and is demo mode (is not demo - return false)', function () {
            ctrl.showFilterIcon = 'false';

            ctrl.isShowFilterActionIcon();

            if (ConfigService.isDemoMode()) {
                expect(ctrl.isShowFilterActionIcon()).toBeTruthy();
            } else {
                expect(ctrl.isShowFilterActionIcon()).toBeFalsy();
            }
        });

        it('should return false if filter toggle method does\'t exist', function () {
            ctrl.filtersToggleMethod = 'not a function';

            ctrl.isShowFilterActionIcon();

            expect(ctrl.isShowFilterActionIcon()).toBeFalsy();
        });
    });
});

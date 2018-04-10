describe('nclHeader Component:', function () {
    var $componentController;
    var $httpBackend;
    var $rootScope;
    var $state;
    var $q;
    var ConfigService;
    var NavigationTabsService;
    var UsersDataService;
    var ctrl;

    beforeEach(function () {
        module('nuclio.app');

        inject(function (_$componentController_, _$rootScope_, _$httpBackend_, _$state_, _$q_, _ConfigService_, _NavigationTabsService_, _UsersDataService_) {
            $componentController = _$componentController_;
            $rootScope = _$rootScope_;
            $httpBackend = _$httpBackend_;
            $state = _$state_;
            $q = _$q_;
            ConfigService = _ConfigService_;
            NavigationTabsService = _NavigationTabsService_;
            UsersDataService = _UsersDataService_;
        });

        // Mock request for Tags component
        var userId = '66b6fd2e-be70-477c-8226-7991757c0d79';
        var baseUrl = ConfigService.url.api.baseUrl;
        var element = angular.element('<header></header>');

        $httpBackend
            .whenGET(baseUrl + '/users/' + userId)
            .respond({
                data: {
                    "type": "user",
                    "id": "66b6fd2e-be70-477c-8226-7991757c0d79",
                    "attributes": {
                        "avatar": "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"
                    }
                }
            });

        spyOn(UsersDataService, 'getUser').and.returnValue($q.when({
            type: 'user',
            id: '66b6fd2e-be70-477c-8226-7991757c0d79',
            attr: {
                'first_name': 'Iguazio',
                'last_name': 'Administrator',
                username: 'admin'
            }
        }));

        $state.current = {
            data: {
                mainHeaderTitle: 'Test title!'
            }
        };

        ctrl = $componentController('igzHeader', {$state: $state, $element: element});
        ctrl.$onInit();
    });

    afterEach(function () {
        $componentController = null;
        $rootScope = null;
        $state = null;
        $q = null;
        ctrl = null;
        ConfigService = null;
        NavigationTabsService = null;
        UsersDataService = null;
    });

    describe('getUserAvatarTooltip()', function () {
        it('should get tooltip with full name and username for user`s avatar', function () {
            $rootScope.$digest();

            expect(ctrl.userAvatarTooltip).toEqual('Iguazio Administrator (admin)');
        });
    });

    describe('onToggleHeader()', function () {
        it('should expand header', function () {
            ctrl.isHeaderExpanded = false;

            ctrl.onToggleHeader();

            expect(ctrl.isHeaderExpanded).toBeTruthy();
        });

        it('should collapse header', function () {
            ctrl.isHeaderExpanded = true;

            ctrl.onToggleHeader();

            expect(ctrl.isHeaderExpanded).toBeFalsy();
        });
    });
});
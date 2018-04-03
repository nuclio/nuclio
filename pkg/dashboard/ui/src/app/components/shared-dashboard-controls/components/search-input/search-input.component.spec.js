describe('igzSearchInput Component:', function () {
    var $componentController;
    var $rootScope;
    var $timeout;
    var SearchHelperService;
    var ctrl;

    beforeEach(function () {
        module('iguazio.app');

        inject(function (_$componentController_, _$rootScope_, _$timeout_, _SearchHelperService_) {
            $componentController = _$componentController_;
            $rootScope = _$rootScope_;
            $timeout = _$timeout_;
            SearchHelperService = _SearchHelperService_;
        });

        var bindings = {
            searchStates: {},
            searchKeys: ['attr.name'],
            dataSet: [
                {
                    attr: {
                        name: '1'
                    },
                    ui: {
                        children: ''
                    }
                },
                {
                    attr: {
                        name: '2'
                    },
                    ui: {
                        children: ''
                    }
                },
                {
                    attr: {
                        name: '3'
                    },
                    ui: {
                        children: ''
                    }
                }
            ]
        };

        ctrl = $componentController('igzSearchInput', null, bindings);
        ctrl.$onInit();
    });

    afterEach(function () {
        $componentController = null;
        $rootScope = null;
        $timeout = null;
        SearchHelperService = null;
        ctrl = null;
    });

    describe('onPressEnter()', function () {
        it('should called onPressEnter', function () {
            spyOn(ctrl, 'onPressEnter');
            ctrl.onPressEnter();

            expect(ctrl.onPressEnter).toHaveBeenCalled();
        });
        it('should call makeSearch() method', function () {
            var e = {keyCode: 13};
            spyOn(SearchHelperService, 'makeSearch');
            ctrl.onPressEnter(e);

            expect(SearchHelperService.makeSearch).toHaveBeenCalled();
        });
    });

    describe('onChangeSearchQuery()', function () {
        it('should call makeSearch() after changing searchQuery', function () {
            spyOn(SearchHelperService, 'makeSearch');
            $rootScope.$digest();

            ctrl.searchQuery = 'new';
            $rootScope.$digest();

            $timeout(function () {
                expect(SearchHelperService.makeSearch).toHaveBeenCalled();
            });
            $timeout.flush();
        });
    });

    describe('onDataChanged()', function () {
        it('should call makeSearch() after sending broadcast', function () {
            spyOn(SearchHelperService, 'makeSearch');
            $rootScope.$broadcast('search-input_refresh-search');

            $timeout(function () {
                expect(SearchHelperService.makeSearch).toHaveBeenCalled();
            });
            $timeout.flush();
        });
    });

    describe('resetSearch()', function () {
        it('should call makeSearch() after sending broadcast', function () {
            spyOn(SearchHelperService, 'makeSearch');
            $rootScope.$broadcast('search-input_reset');

            $timeout(function () {
                expect(SearchHelperService.makeSearch).toHaveBeenCalled();
            });
            $timeout.flush();
        });
    });

    describe('clearInputField()', function () {
       it('should empty search field after call clearInputField()', function () {
           ctrl.searchQuery = 'new';
           ctrl.clearInputField();
           expect(ctrl.searchQuery).toEqual('');
       });
    });
});
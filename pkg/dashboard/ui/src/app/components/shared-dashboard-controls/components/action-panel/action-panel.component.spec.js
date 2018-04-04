describe('igzActionPanel component:', function () {
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
            actions: [
                {
                    label: 'Download',
                    id: 'default',
                    icon: 'download',
                    active: true,
                    callback: false
                },
                {
                    label: 'Delete',
                    id: 'delete',
                    icon: 'trash',
                    active: true,
                    callback: false
                },
                {
                    label: 'Clone',
                    id: 'clone',
                    icon: 'multidoc',
                    active: true,
                    callback: false
                },
                {
                    label: 'Snapshot',
                    id: 'snapshot',
                    icon: 'camera',
                    active: true,
                    callback: function () {
                        $rootScope.$broadcast('passed');
                    }
                },
                {
                    label: 'Properties',
                    id: 'default',
                    icon: 'note',
                    active: true,
                    callback: false
                },
                {
                    label: 'Info',
                    id: 'toggleInfoPanel',
                    icon: 'info',
                    active: true,
                    callback: false
                }
            ]
        };

        ctrl = $componentController('igzActionPanel', null, bindings);

        ctrl.$onInit();
    });

    afterEach(function () {
        $componentController = null;
        $rootScope = null;
        ctrl = null;
    });

    describe('isActionPanelShown()', function () {
        it('should return true if count of checked item is more than 0', function () {
            $rootScope.$broadcast('action-checkbox-all_checked-items-count-change', {
                checkedCount: 5
            });
            $rootScope.$digest();
            expect(ctrl.isActionPanelShown()).toBeTruthy();
        });

        it('should return false if count of checked item is less than 1', function () {
            $rootScope.$broadcast('action-checkbox-all_checked-items-count-change', {
                checkedCount: 0
            });
            $rootScope.$digest();
            expect(ctrl.isActionPanelShown()).toBeFalsy();
        });
    });
});

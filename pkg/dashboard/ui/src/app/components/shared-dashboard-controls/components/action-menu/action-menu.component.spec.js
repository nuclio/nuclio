describe('igzActionMenu component:', function () {
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
            ],
            onFireAction: function () {
            }
        };
        var element = '<igz-action-menu></igz-action-menu>';

        ctrl = $componentController('igzActionMenu', {$element: element}, bindings);
    });

    afterEach(function () {
        $componentController = null;
        $rootScope = null;
        ctrl = null;
    });

    describe('toggleMenu()', function () {
        it('should change value of boolean variable isMenuShown', function () {
            var event = {
                stopPropagation: angular.noop
            };
            expect(ctrl.isMenuShown).toBeFalsy();
            ctrl.toggleMenu(event);
            expect(ctrl.isMenuShown).toBeTruthy();
        });
    });

    describe('showDetails(): ', function () {
        it('should call ctrl.onClickShortcut()', function () {
            ctrl.onClickShortcut = angular.noop;
            var event = {
                preventDefault: angular.noop,
                stopPropagation: angular.noop
            };
            spyOn(ctrl, 'onClickShortcut');

            ctrl.showDetails(event, 'state');

            expect(ctrl.onClickShortcut).toHaveBeenCalled();
        });
    });

    describe('isVisible(): ', function () {
        it('should return true if there are action menu items', function () {
            expect(ctrl.isVisible()).toBeTruthy();
        });

        it('should return false if there is no action menu items', function () {
            ctrl.actions = null;
            expect(ctrl.isVisible()).toBeFalsy();
        });

        it('should return true if there are action menu shortcuts', function () {
            ctrl.actions = null;
            ctrl.shortcuts = [
                {
                    label: 'shortcutLabel',
                    state: 'state'
                }
            ];
            expect(ctrl.isVisible()).toBeTruthy();
        });
    });
});

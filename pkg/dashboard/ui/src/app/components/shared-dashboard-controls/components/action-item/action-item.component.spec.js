describe('igzActionItem component:', function () {
    var $componentController;
    var $rootScope;
    var ctrl;
    var ngDialog;
    var ngDialogSpy;

    beforeEach(function () {
        module('iguazio.app');

        inject(function (_$componentController_, _$rootScope_, _ngDialog_) {
            $componentController = _$componentController_;
            $rootScope = _$rootScope_;
            ngDialog = _ngDialog_;
        });

        ngDialogSpy = spyOn(ngDialog, 'openConfirm').and.returnValue({
            then: function (thenCallback) {
                thenCallback();
            }
        });

        var bindings = {
            action: {
                label: 'Download',
                id: 'default',
                icon: 'download',
                active: true,
                confirm: {
                    message: 'Are you sure you want to delete selected items?',
                    yesLabel: 'Yes, Delete',
                    noLabel: 'Cancel',
                    type: 'critical_alert'
                },
                template: '<div></div>',
                subTemplateProps: {
                    isShown: false
                },
                handler: function () {
                },
                callback: function () {
                }
            }
        };
        var element = '<igz-action-item></igz-action-item>';

        ctrl = $componentController('igzActionItem', {$element: element}, bindings);
    });

    afterEach(function () {
        $componentController = null;
        $rootScope = null;
        ctrl = null;
        ngDialog = null;
        ngDialogSpy = null;
    });

    describe('onClickAction()', function () {
        it('should just call action.handler if action.confirm is not defined', function () {
            var spy = spyOn(ctrl.action, 'handler');
            ctrl.onClickAction();
            expect(spy).toHaveBeenCalled();
        });

        it('should show confirm dialog and then call action.handler if action.confirm is defined', function () {
            var spy = spyOn(ctrl.action, 'handler');
            ctrl.onClickAction();
            expect(ngDialogSpy).toHaveBeenCalled();
            expect(spy).toHaveBeenCalled();
        });

        it('should show subtemplate if action.template is defined', function () {
            expect(ctrl.action.subTemplateProps.isShown).toBeFalsy();
            ctrl.onClickAction();
            expect(ctrl.action.subTemplateProps.isShown).toBeTruthy();
        });

        it('should call action.callback if defined', function () {
            var spy = spyOn(ctrl.action, 'callback');
            ctrl.onClickAction();
            expect(spy).toHaveBeenCalled();
        });

        it('should not call action event if action.active \'false\'', function () {
            ctrl.action.active = false;

            var spyHandler = spyOn(ctrl.action, 'handler');
            var spyCallback = spyOn(ctrl.action, 'callback');

            ctrl.onClickAction();

            expect(spyHandler).not.toHaveBeenCalled();
            expect(spyCallback).not.toHaveBeenCalled();
        });
    });
});

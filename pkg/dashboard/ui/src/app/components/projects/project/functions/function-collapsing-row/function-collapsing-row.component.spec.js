describe('nclFunctionCollapsingRow component:', function () {
    var $componentController;
    var ctrl;
    var functionItem;

    beforeEach(function () {
        module('iguazio.app');

        inject(function (_$componentController_) {
            $componentController = _$componentController_;
        });

        functionItem = {
            'metadata': {
                'name': 'functionName',
                'namespace': 'nuclio'
            },
            'spec': {
                'description': 'Some description',
                'runtime': 'golang',
                'replicas': 1,
                'build': {},
                'runRegistry': 'localhost:5000'
            }
        };

        var bindings = {
            function: functionItem,
            actionHandlerCallback: angular.noop
        };

        ctrl = $componentController('nclFunctionCollapsingRow', null, bindings);

        ctrl.$onInit();
    });

    afterEach(function () {
        $componentController = null;
        ctrl = null;
        functionItem = null;
    });

    describe('$onInit(): ', function () {
        it('should set initial values for actions and delete function method', function () {
            expect(ctrl.function.ui.delete).not.toBeUndefined();
            expect(ctrl.actions).not.toBe([]);
        });
    });

    describe('onFireAction(): ', function () {
        it('should call actionHandlerCallback() method', function () {
            spyOn(ctrl, 'actionHandlerCallback');

            ctrl.onFireAction('delete');

            expect(ctrl.actionHandlerCallback).toHaveBeenCalled();
        });
    });

    describe('isFunctionShowed(): ', function () {
        it('should return true if function is showed', function () {
            ctrl.function.ui.isShowed = true;

            expect(ctrl.isFunctionShowed()).toBeTruthy();
        });
    });

    describe('handleAction(): ', function () {
        it('should call actionHandlerCallback() method', function () {
            spyOn(ctrl, 'actionHandlerCallback');

            ctrl.handleAction('delete', []);

            expect(ctrl.actionHandlerCallback).toHaveBeenCalled();
        });
    });
});
describe('nclFunctionVersionRow component:', function () {
    var $componentController;
    var ctrl;
    var functionItem;
    var project;
    var version;

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
        project = {
            "metadata": {
                "name": "my-project-1",
                "namespace": "nuclio"
            },
            "spec": {
                "displayName": "My project #1",
                "description": "Some description"
            }
        };
        version = {
            name: 'Version 1',
            invocation: '30',
            last_modified: '2018-02-05T17:07:48.509Z'
        };

        var bindings = {
            version: version,
            project: project,
            function: functionItem,
            versionsList: [version],
            actionHandlerCallback: angular.noop
        };

        ctrl = $componentController('nclFunctionVersionRow', null, bindings);

        ctrl.$onInit();
    });

    afterEach(function () {
        $componentController = null;
        ctrl = null;
        functionItem = null;
        project = null;
        version = null;
    });

    describe('$onInit(): ', function () {
        it('should set initial values for actions and ui fields', function () {
            expect(ctrl.version.ui.checked).toBeFalsy();
            expect(ctrl.version.ui.edit).not.toBeUndefined();
            expect(ctrl.version.ui.delete).not.toBeUndefined();

            expect(ctrl.actions).not.toBe([]);
        });
    });

    describe('onFireAction(): ', function () {
        it('should call actionHandlerCallback() method', function () {
            spyOn(ctrl, 'actionHandlerCallback');

            ctrl.onFireAction('delete');

            expect(ctrl.actionHandlerCallback).toHaveBeenCalled();
            expect(ctrl.actionHandlerCallback).toHaveBeenCalledWith({actionType: 'delete', checkedItems: [ctrl.version]});
        });
    });
});
describe('nclEditProjectDialog component:', function () {
    var $componentController;
    var $q;
    var $rootScope;
    var NuclioProjectsDataService;
    var ctrl;
    var scope;
    var project;

    beforeEach(function () {
        module('iguazio.app');

        inject(function (_$componentController_, _$q_, _$rootScope_, _NuclioProjectsDataService_) {
            $componentController = _$componentController_;
            $q = _$q_;
            $rootScope = _$rootScope_;
            NuclioProjectsDataService = _NuclioProjectsDataService_;
        });

        scope = $rootScope.$new();
        scope.editProjectForm = {
            $valid: true
        };
        project = {
            metadata: {
                name: 'my-project-1',
                namespace: 'nuclio'
            },
            spec: {
                displayName: 'My project #1',
                description: 'Some description'
            }
        };
        var bindings = {
            project: project,
            confirm: angular.noop,
            closeDialog: angular.noop
        };

        ctrl = $componentController('nclEditProjectDialog', {$scope: scope}, bindings);
        ctrl.$onInit();
    });

    afterEach(function () {
        $componentController = null;
        $q = null;
        $rootScope = null;
        ctrl = null;
        NuclioProjectsDataService = null;
    });

    describe('$onInit(): ', function () {
        it('should set copy of `ctrl.project` to `ctrl.data`', function () {
            expect(ctrl.data).toEqual(project);
        });
    });

    describe('saveProject(): ', function () {
        it('should resolve NuclioProjectsDataService.updateProject() method if form is valid', function () {
            spyOn(NuclioProjectsDataService, 'updateProject').and.callFake(function () {
                return $q.resolve();
            });
            spyOn(ctrl, 'confirm').and.callThrough();

            ctrl.saveProject();
            $rootScope.$digest();

            expect(NuclioProjectsDataService.updateProject).toHaveBeenCalledWith(ctrl.data);
            expect(ctrl.confirm).toHaveBeenCalled();
            expect(ctrl.serverError).toBe('');
        });

        // todo error status cases
        it('should reject NuclioProjectsDataService.updateProject() method if there is error' +
            '(missing mandatory fields) is response', function () {
            spyOn(NuclioProjectsDataService, 'updateProject').and.callFake(function () {
                return $q.reject({
                    data: {
                        errors: [
                            {
                                status: 400
                            }
                        ]
                    }
                });
            });

            ctrl.saveProject();
            $rootScope.$digest();

            expect(NuclioProjectsDataService.updateProject).toHaveBeenCalledWith(ctrl.data);
            expect(ctrl.serverError).toBe('Missing mandatory fields');
        });
    });

    describe('inputValueCallback(): ', function () {
        it('should set new value from input to `name` field', function () {
            var expectedName = 'new name';

            ctrl.inputValueCallback(expectedName, 'spec.displayName');

            expect(ctrl.data.spec.displayName).toBe(expectedName);
        });

        it('should set new value from input to `description` field', function () {
            var expectedDescription = 'new description';

            ctrl.inputValueCallback(expectedDescription, 'spec.description');

            expect(ctrl.data.spec.description).toBe(expectedDescription);
        });
    });

    describe('onClose(): ', function () {
        it('should close dialog calling closeDialog() method', function () {
            spyOn(ctrl, 'closeDialog');

            ctrl.onClose();

            expect(ctrl.closeDialog).toHaveBeenCalled();
        });
    });
});
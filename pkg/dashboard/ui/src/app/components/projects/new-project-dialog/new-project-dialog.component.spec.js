describe('nclNewProjectDialog component:', function () {
    var $componentController;
    var $q;
    var $rootScope;
    var NuclioProjectsDataService;
    var ctrl;
    var scope;

    beforeEach(function () {
        module('iguazio.app');

        inject(function (_$componentController_, _$q_, _$rootScope_, _NuclioProjectsDataService_) {
            $componentController = _$componentController_;
            $q = _$q_;
            $rootScope = _$rootScope_;
            NuclioProjectsDataService = _NuclioProjectsDataService_;
        });

        scope = $rootScope.$new();
        scope.newProjectForm = {
            $valid: true
        };
        var bindings = {
            closeDialog: angular.noop
        };

        ctrl = $componentController('nclNewProjectDialog', {$scope: scope}, bindings);

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
        it('should set black data', function () {
            var data = {
                metadata: {
                    namespace: ''
                },
                spec: {
                    displayName: '',
                    description: ''
                }
            };

            expect(ctrl.data).toEqual(data);
        });
    });

    describe('createProject(): ', function () {
        it('should resolve NuclioProjectsDataService.createProject() method if form is valid', function () {
            spyOn(NuclioProjectsDataService, 'createProject').and.callFake(function () {
                return $q.resolve();
            });
            spyOn(ctrl, 'closeDialog').and.callThrough();

            var blankData = {
                metadata: {
                    namespace: ''
                },
                spec: {
                    displayName: '',
                    description: ''
                }
            };
            var newData = {
                metadata: {
                    namespace: 'nuclio'
                },
                spec: {
                    displayName: 'project-1',
                    description: 'project-description',
                    created_by: 'admin',
                    created_date: '2017-06-20T08:15:56.000Z'
                }
            };
            ctrl.data = newData;

            ctrl.createProject();
            $rootScope.$digest();

            expect(NuclioProjectsDataService.createProject).toHaveBeenCalledWith(newData);
            expect(ctrl.closeDialog).toHaveBeenCalled();
            expect(ctrl.data).toEqual(blankData);
        });

        // todo error status cases
        it('should reject NuclioProjectsDataService.createProject() method if there is error' +
            '(missing mandatory fields) is response', function () {
            spyOn(NuclioProjectsDataService, 'createProject').and.callFake(function () {
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

            ctrl.data = {
                metadata: {
                    namespace: 'nuclio'
                },
                spec: {
                    displayName: 'project-1',
                    description: 'project-description',
                    created_by: 'admin',
                    created_date: '2017-06-20T08:15:56.000Z'
                }
            };

            ctrl.createProject();
            $rootScope.$digest();

            expect(NuclioProjectsDataService.createProject).toHaveBeenCalledWith(ctrl.data);
            expect(ctrl.serverError).toBe('Missing mandatory fields');
        });
    });

    describe('inputValueCallback(): ', function () {
        it('should set new value from input to `name` field', function () {
            var expectedName = 'new name';

            ctrl.inputValueCallback(expectedName, 'spec.displayName');

            expect(ctrl.data.spec.displayName).toBe(expectedName);
        });

        it('should set new value from input to `name` field', function () {
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
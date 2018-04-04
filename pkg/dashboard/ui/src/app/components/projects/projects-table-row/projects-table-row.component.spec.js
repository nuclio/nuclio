describe('nclProjectsTableRow component:', function () {
    var $componentController;
    var $q;
    var $rootScope;
    var ActionCheckboxAllService;
    var DialogsService;
    var NuclioProjectsDataService;
    var ctrl;
    var project;
    var projectsList;

    beforeEach(function () {
        module('iguazio.app');

        inject(function (_$componentController_, _$q_, _$rootScope_, _ActionCheckboxAllService_, _DialogsService_, _NuclioProjectsDataService_) {
            $rootScope = _$rootScope_;
            $componentController = _$componentController_;
            $q = _$q_;
            ActionCheckboxAllService = _ActionCheckboxAllService_;
            DialogsService = _DialogsService_;
            NuclioProjectsDataService = _NuclioProjectsDataService_;
        });

        project = {
            metadata: {
                name: 'my-project-1',
                namespace: 'nuclio'
            },
            spec: {
                displayName: 'My project #1',
                description: 'Some description'
            },
            ui: {
                checked: false
            }
        };
        projectsList = [
            {
                metadata: {
                    name: 'my-project-1',
                    namespace: 'nuclio'
                },
                spec: {
                    displayName: 'My project #1',
                    description: 'Some description'
                },
                ui: {
                    checked: false
                }
            },
            {
                metadata: {
                    name: 'my-project-2',
                    namespace: 'nuclio'
                },
                spec: {
                    displayName: 'My project #2',
                    description: 'Some description'
                },
                ui: {
                    checked: false
                }
            }
        ];
        var bindings = {
            project: project,
            projectsList: angular.copy(projectsList)
        };

        ctrl = $componentController('nclProjectsTableRow', null, bindings);
    });

    afterEach(function () {
        $componentController = null;
        $q = null;
        $rootScope = null;
        ctrl = null;
        ActionCheckboxAllService = null;
        DialogsService = null;
        NuclioProjectsDataService = null;
    });

    describe('$onInit(): ', function () {
        it('should initialize `deleteProject`, `editProjects` actions and assign them to `ui` property of current project' +
           'should initialize `checked` status to `false`', function () {
            ctrl.$onInit();

            expect(ctrl.project.ui.checked).toBeFalsy();
        });
    });

    describe('showDetails(): ', function () {

        // todo
    });

    describe('deleteProject(): ', function () {
        it('should resolve NuclioProjectsDataService.deleteProject() method if there is error' +
            '(missing mandatory fields) is response', function () {
            spyOn(NuclioProjectsDataService, 'deleteProject').and.callFake(function () {
                return $q.resolve();
            });

            ctrl.$onInit();
            ctrl.project.ui.delete();
            $rootScope.$digest();
            project.ui = ctrl.project.ui;

            expect(NuclioProjectsDataService.deleteProject).toHaveBeenCalledWith(project);
        });

        // todo error status cases
        it('should resolve NuclioProjectsDataService.deleteProject() method if there is error' +
            '(missing mandatory fields) is response', function () {
            spyOn(NuclioProjectsDataService, 'deleteProject').and.callFake(function () {
                return $q.reject({
                    status: 403
                });
            });
            spyOn(DialogsService, 'alert');

            ctrl.$onInit();
            ctrl.project.ui.delete();
            $rootScope.$digest();
            project.ui = ctrl.project.ui;

            expect(NuclioProjectsDataService.deleteProject).toHaveBeenCalledWith(project);
            expect(DialogsService.alert).toHaveBeenCalledWith('You do not have permissions to delete this project.');
        });
    });

    describe('editProject(): ', function () {
        // todo
    });
});
describe('nclProjects component: ', function () {
    var $componentController;
    var $rootScope;
    var $q;
    var ngDialog;
    var ctrl;
    var hasCapabilitiesResult;
    var NuclioProjectsDataService;
    var mockData;
    var projects;

    beforeEach(function () {
        module('iguazio.app');

        inject(function (_$rootScope_, _$componentController_, _$q_, _ngDialog_, _NuclioProjectsDataService_) {
            $rootScope = _$rootScope_;
            $componentController = _$componentController_;
            $q = _$q_;
            ngDialog = _ngDialog_;
            NuclioProjectsDataService = _NuclioProjectsDataService_;

            mockData = {
                'my-project-1': {
                    metadata: {
                        name: 'my-project-1',
                        namespace: 'nuclio'
                    },
                    spec: {
                        displayName: 'My project #1',
                        description: 'Some description'
                    },
                    ui: {
                        checked: false,
                        delete: angular.noop
                    }
                },
                'my-project-2': {
                    metadata: {
                        name: 'my-project-2',
                        namespace: 'nuclio'
                    },
                    spec: {
                        displayName: 'My project #2',
                        description: 'Some description'
                    },
                    ui: {
                        checked: false,
                        delete: angular.noop
                    }
                }
            };
            projects = [
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
                        checked: false,
                        delete: angular.noop
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
                        checked: false,
                        delete: angular.noop
                    }
                }
            ];

            hasCapabilitiesResult = true;
            spyOn(NuclioProjectsDataService, 'getProjects').and.callFake(function () {
                return $q.when(mockData);
            });

            ctrl = $componentController('nclProjects');
        });
    });

    afterEach(function () {
        $componentController = null;
        $rootScope = null;
        $q = null;
        ngDialog = null;
        ctrl = null;
        hasCapabilitiesResult = null;
        NuclioProjectsDataService = null;
        mockData = null;
    });

    describe('$onInit(): ', function () {
        it('should set "ctrl.readOnly" to the "false" if session has "nuclio.projects.edit" capability', function () {
            hasCapabilitiesResult = true;

            ctrl.$onInit();

            expect(ctrl.readOnly).toBeFalsy();
        });

        it('should set "ctrl.readOnly" to the "true" if session does not have "nuclio.projects.edit" capability', function () {
            hasCapabilitiesResult = false;

            ctrl.$onInit();

            expect(ctrl.readOnly).toBeTruthy();
        });

        it('should initialize projects actions array', function () {
            var expectedActions = [
                {
                    label: 'Delete',
                    id: 'delete',
                    icon: 'igz-icon-trash',
                    active: true,
                    capability: 'nuclio.projects.delete',
                    confirm: {
                        message: 'Delete selected projects?',
                        yesLabel: 'Yes, Delete',
                        noLabel: 'Cancel',
                        type: 'nuclio_alert'
                    }
                },
                {
                    label: 'Edit',
                    id: 'edit',
                    icon: 'igz-icon-properties',
                    active: true,
                    capability: 'nuclio.projects.edit'
                }
            ];

            ctrl.$onInit();

            expect(ctrl.actions).toEqual(expectedActions);
        });

        it('should initialize projects array', function () {
            ctrl.$onInit();
            $rootScope.$digest();

            expect(ctrl.projects).toEqual(projects);
        });

        it('should call onFireAction() method if `action-panel_fire-action` broadcast was sent', function () {
            spyOn(ctrl, 'handleAction');

            ctrl.$onInit();
            $rootScope.$digest();

            var data = {
                action: {
                    label: 'Edit',
                    id: 'edit',
                    icon: 'igz-icon-properties',
                    active: true,
                    capability: 'nuclio.projects.edit'
                }
            };
            ctrl.projects[0].ui = {
                checked: true
            };
            projects[0].ui = ctrl.projects[0].ui;

            $rootScope.$broadcast('action-panel_fire-action', data);

            $rootScope.$digest();

            expect(ctrl.handleAction).toHaveBeenCalledWith(data.action, [projects[0]]);
        });
    });

    describe('$onDestroy(): ', function () {
        it('should close opened ngDialog', function () {
            spyOn(ngDialog, 'close');

            ctrl.$onDestroy();

            expect(ngDialog.close).toHaveBeenCalled();
        });
    });

    describe('handleAction(): ', function () {
        it('should call action`s handlers for all checked projects', function () {
            var data = {
                action: {
                    label: 'Delete',
                    id: 'delete',
                    icon: 'igz-icon-trash',
                    active: true,
                    capability: 'nuclio.projects.delete',
                    confirm: {
                        message: 'Delete selected projects?',
                        yesLabel: 'Yes, Delete',
                        noLabel: 'Cancel',
                        type: 'nuclio_alert'
                    }
                }
            };

            ctrl.$onInit();
            $rootScope.$digest();

            ctrl.projects[1].ui.checked = true;
            projects[1].ui = ctrl.projects[1].ui;

            spyOn(ctrl.projects[0].ui, 'delete');
            spyOn(ctrl.projects[1].ui, 'delete');

            ctrl.handleAction(data.action.id, [ctrl.projects[0], ctrl.projects[1]]);

            expect(ctrl.projects[0].ui.delete).toHaveBeenCalled();
            expect(ctrl.projects[1].ui.delete).toHaveBeenCalled();
        });
    });

    describe('openNewProjectDialog(): ', function () {
        it('should open ngDialog', function () {
            spyOn(ngDialog, 'open').and.returnValue({
                closePromise : {
                    then : function(callback) {
                        callback();
                    }
                }
            });

            ctrl.openNewProjectDialog();

            expect(ngDialog.open).toHaveBeenCalled();
            expect(NuclioProjectsDataService.getProjects).toHaveBeenCalled();
        });
    });

    describe('refreshProjects(): ', function () {
        it('should change value of `ctrl.isFiltersShowed`', function () {
            ctrl.refreshProjects();

            $rootScope.$digest();

            expect(NuclioProjectsDataService.getProjects).toHaveBeenCalled();
        });
    });

    describe('toggleFilters(): ', function () {
        it('should change value of `ctrl.isFiltersShowed`', function () {
            ctrl.isFiltersShowed.value = false;

            ctrl.toggleFilters();

            expect(ctrl.isFiltersShowed.value).toBeTruthy();

            ctrl.toggleFilters();

            expect(ctrl.isFiltersShowed.value).toBeFalsy();
        });
    });
});
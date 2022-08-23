/*
Copyright 2017 The Nuclio Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
describe('nclProjects component: ', function () {
    var $componentController;
    var $q;
    var $rootScope;
    var $state;
    var ngDialog;
    var ExportService;
    var ProjectsService;
    var ctrl;
    var projects;
    var sortOptions;

    beforeEach(function () {
        module('nuclio.app');

        inject(function (_$componentController_, _$q_, _$rootScope_, _$state_, _ngDialog_, _ExportService_,
                         _ProjectsService_) {
            $componentController = _$componentController_;
            $q = _$q_;
            $rootScope = _$rootScope_;
            $state = _$state_;
            ngDialog = _ngDialog_;
            ExportService = _ExportService_;
            ProjectsService = _ProjectsService_;

            projects = [
                {
                    metadata: {
                        name: 'my-project-1',
                        namespace: 'nuclio'
                    },
                    spec: {
                        description: 'Some description'
                    },
                    ui: {
                        functions: [],
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
                        description: 'Some description'
                    },
                    ui: {
                        functions: [],
                        checked: false,
                        delete: angular.noop
                    }
                }
            ];
            sortOptions = [
                {
                    label: 'Name',
                    value: 'metadata.name',
                    active: true
                },
                {
                    label: 'Description',
                    value: 'spec.description',
                    active: false
                }
            ];

            var bindings = {
                projects: projects,
                getProject: $q.when.bind($q),
                getProjects: $q.when.bind($q),
                getFunctions: $q.when.bind($q)
            };
            var element = angular.element('<ncl-projects></ncl-projects>');
            var ImportService = {
                importFile: angular.noop
            };

            ctrl = $componentController('nclProjects', {$element: element, ImportService: ImportService}, bindings);
            ctrl.$onInit();
            $rootScope.$digest();
        });
    });

    afterEach(function () {
        $componentController = null;
        $q = null;
        $rootScope = null;
        $state = null;
        ngDialog = null;
        ExportService = null;
        ProjectsService = null;
        ctrl = null;
        projects = null;
        sortOptions = null;
    });

    describe('$onInit(): ', function () {
        it('should initialize projects array', function () {
            expect(ctrl.projects).toEqual(projects);
        });

        it('should initialize sort options', function () {
            expect(ctrl.sortOptions).not.toBe([]);
        });

        it('should call onFireAction() method if `action-panel_fire-action` broadcast was sent', function () {
            spyOn(ctrl, 'handleProjectAction');

            var data = {
                action: {
                    label: 'EDIT',
                    id: 'edit',
                    icon: 'igz-icon-properties',
                    active: true
                }
            };
            ctrl.projects[0].ui = {
                checked: true
            };
            projects[0].ui = ctrl.projects[0].ui;

            $rootScope.$broadcast('action-panel_fire-action', data);

            $rootScope.$digest();

            expect(ctrl.handleProjectAction).toHaveBeenCalledWith(data.action, [projects[0]]);
        });
    });

    describe('handleProjectAction(): ', function () {
        it('should call action\'s handlers for all checked projects', function () {
            var mockedValue = {
                then: function () {
                    return {
                        'catch': angular.noop
                    }
                }
            };
            ctrl.projects[1].ui.checked = true;
            projects[1].ui = ctrl.projects[1].ui;

            spyOn(ctrl.projects[0].ui, 'delete').and.returnValue(mockedValue);
            spyOn(ctrl.projects[1].ui, 'delete').and.returnValue(mockedValue);

            ctrl.handleProjectAction('delete', [ctrl.projects[0], ctrl.projects[1]]);

            expect(ctrl.projects[0].ui.delete).toHaveBeenCalled();
            expect(ctrl.projects[1].ui.delete).toHaveBeenCalled();
        });
    });

    describe('isProjectsListEmpty(): ', function () {
        it('should return true if projects list in empty', function () {
            ctrl.projects = [];

            expect(ctrl.isProjectsListEmpty()).toBeTruthy();
        });

        it('should return false if projects list in not empty', function () {
            ctrl.projects = projects;

            expect(ctrl.isProjectsListEmpty()).toBeFalsy();
        });
    });

    describe('onApplyFilters(): ', function () {
        it('should call `search-input_refresh-search` broadcast', function () {
            spyOn($rootScope, '$broadcast').and.callThrough();

            ctrl.onApplyFilters();

            expect($rootScope.$broadcast).toHaveBeenCalledWith('search-input_refresh-search');
        });
    });

    describe('onResetFilters(): ', function () {
        it('should call `search-input_reset` broadcast', function () {
            spyOn($rootScope, '$broadcast').and.callThrough();
            ctrl.filtersCounter = 1;

            ctrl.onResetFilters();

            expect($rootScope.$broadcast).toHaveBeenCalledWith('search-input_reset');
            expect(ctrl.filtersCounter).toEqual(0)
        });
    });

    describe('onSelectDropdownAction(): ', function () {
        it('should call `onSelectDropdown` function', function () {
            spyOn(ctrl, 'onSelectDropdownAction');

            ctrl.onSelectDropdownAction({id: 'exportProjects'});

            expect(ctrl.onSelectDropdownAction).toHaveBeenCalled();
        });

        it('should call `exportProject` handler', function () {
            spyOn(ExportService, 'exportProjects');

            ctrl.onSelectDropdownAction({id: 'exportProjects'});

            expect(ExportService.exportProjects).toHaveBeenCalledWith(jasmine.arrayContaining([jasmine.objectContaining({
                metadata: jasmine.objectContaining({
                    name: jasmine.any(String)
                })
            })]), jasmine.any(Function));
        });
    });

    describe('onSortOptionsChange(): ', function () {
        it('should set `sortedColumnName` and `isReverseSorting` according to selected option, and sort projects', function () {
            var option = {
                value: 'metadata.name',
                desc: true
            };
            ctrl.projects = ctrl.sortedProjects = [
                {
                    metadata: {
                        name: 'name1'
                    }
                },
                {
                    metadata: {
                        name: 'name2'
                    }
                }
            ];

            ctrl.onSortOptionsChange(option);

            expect(ctrl.sortedColumnName).toEqual('metadata.name');
            expect(ctrl.isReverseSorting).toEqual(true);
            expect(ctrl.sortedProjects).toEqual([
                {
                    metadata: {
                        name: 'name2'
                    }
                },
                {
                    metadata: {
                        name: 'name1'
                    }
                }
            ]);
        });
    });

    describe('onUpdateFiltersCounter(): ', function () {
        it('should set `filterCounter` to 0 if `filterQuery` is empty', function () {
            ctrl.onUpdateFiltersCounter();

            expect(ctrl.filtersCounter).toEqual(0);
        });

        it('should set `filterCounter` to 1 if `filterQuery` is not empty', function () {
            ctrl.onUpdateFiltersCounter('filter query');

            expect(ctrl.filtersCounter).toEqual(1);
        });
    });

    describe('openNewFunctionScreen(): ', function () {
        it('should call `$state.go` method', function () {
            spyOn($state, 'go').and.callThrough();

            ctrl.openNewFunctionScreen();

            expect($state.go).toHaveBeenCalledWith('app.create-function', {
                navigatedFrom: 'projects'
            });
        });
    });

    describe('openNewProjectDialog(): ', function () {
        it('should open ngDialog and get project list', function () {
            spyOn(ctrl, 'getProjects').and.callThrough();
            spyOn(ngDialog, 'open').and.returnValue({
                closePromise: $q.when({
                    value: 'some-value'
                })
            });

            ctrl.openNewProjectDialog();
            $rootScope.$digest();

            expect(ngDialog.open).toHaveBeenCalled();
            expect(ctrl.getProjects).toHaveBeenCalled();
        });
    });

    describe('refreshProjects(): ', function () {
        it('should change value of `ctrl.isFiltersShowed` and call `ctrl.getProjects()`', function () {
            spyOn(ctrl, 'getProjects').and.callThrough();

            ctrl.refreshProjects();
            $rootScope.$digest();

            expect(ctrl.getProjects).toHaveBeenCalled();
        });
    });

    describe('sortTableByColumn(): ', function () {
        it('should set reverse sorting for the same column', function () {
            ctrl.sortedColumnName = 'some-test-column';
            ctrl.isReverseSorting = false;

            ctrl.sortTableByColumn('some-test-column');

            expect(ctrl.sortedColumnName).toBe('some-test-column');
            expect(ctrl.isReverseSorting).toBeTruthy();

            ctrl.sortTableByColumn('some-test-column');

            expect(ctrl.sortedColumnName).toBe('some-test-column');
            expect(ctrl.isReverseSorting).toBeFalsy();
        });

        it('should set sorting for the new column', function () {
            ctrl.sortedColumnName = 'some-test-column';
            ctrl.isReverseSorting = true;

            ctrl.sortTableByColumn('new-column');

            expect(ctrl.sortedColumnName).toBe('new-column');
            expect(ctrl.isReverseSorting).toBeFalsy();
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

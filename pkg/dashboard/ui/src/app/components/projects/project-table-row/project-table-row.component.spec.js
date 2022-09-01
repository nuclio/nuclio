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

describe('nclProjectTableRow component:', function () {
    var $componentController;
    var $q;
    var $rootScope;
    var $state;
    var ngDialog;
    var ExportService;
    var ctrl;
    var project;

    beforeEach(function () {
        module('nuclio.app');

        inject(function (_$componentController_, _$q_, _$rootScope_, _$state_, _ngDialog_,
                         _ExportService_) {
            $componentController = _$componentController_;
            $q = _$q_;
            $rootScope = _$rootScope_;
            $state = _$state_;
            ngDialog = _ngDialog_;
            ExportService = _ExportService_;
        });

        project = {
            metadata: {
                name: 'my-project-1',
                namespace: 'nuclio'
            },
            spec: {
                description: 'Some description'
            },
            ui: {
                checked: false
            }
        };
        var bindings = {
            project: project,
            deleteProject: $q.when.bind($q),
            getFunctions: angular.noop,
            projectActionHandlerCallback: angular.noop
        };

        ctrl = $componentController('nclProjectTableRow', null, bindings);
        ctrl.$onInit();
    });

    afterEach(function () {
        $componentController = null;
        $q = null;
        $rootScope = null;
        $state = null;
        ngDialog = null;
        ctrl = null;
        ExportService = null;
    });

    describe('$onInit(): ', function () {
        it('should initialize `checked` status to `false`', function () {
            expect(ctrl.project.ui.checked).toBeFalsy();
        });

        it('should initialize project actions and their handlers', function () {
            expect(ctrl.project.ui['delete']).toBeDefined();
            expect(ctrl.project.ui['edit']).toBeDefined();
            expect(ctrl.project.ui['export']).toBeDefined();
            expect(ctrl.projectActions).not.toBe({});
        });
    });

    describe('deleteProject(): ', function () {
        it('should resolve `ctrl.deleteProject()` method if there is error ' +
            '(missing mandatory fields) is response', function () {
            spyOn(ctrl, 'deleteProject').and.callThrough();

            ctrl.project.ui.delete();
            $rootScope.$digest();
            project.ui = ctrl.project.ui;

            expect(ctrl.deleteProject).toHaveBeenCalledWith({ project: ctrl.project });
        });

        // todo error status cases
        it('should resolve `ctrl.deleteProject()` method if there is error ' +
            '(missing mandatory fields) is response', function () {
            spyOn(ctrl, 'deleteProject').and.returnValue($q.reject());

            ctrl.project.ui.delete();
            $rootScope.$digest();
            project.ui = ctrl.project.ui;

            expect(ctrl.deleteProject).toHaveBeenCalledWith({ project: ctrl.project });
        });
    });

    describe('editProject(): ', function () {
        it('should call ngDialog.open() method', function () {
            spyOn(ngDialog, 'open').and.returnValue({ closePromise: $q.when() });

            ctrl.project.ui.edit();

            expect(ngDialog.open).toHaveBeenCalled();
        })
    });

    describe('exportProject(): ', function () {
        it('should call ExportService.exportProject() method', function () {
            spyOn(ExportService, 'exportProject');

            ctrl.project.ui.export();

            expect(ExportService.exportProject).toHaveBeenCalledWith(ctrl.project, ctrl.getFunctions);
        });
    });

    describe('onFireAction(): ', function () {
        it('should call projectActionHandlerCallback() method', function () {
            spyOn(ctrl, 'projectActionHandlerCallback');

            ctrl.onFireAction('delete');
            ctrl.onFireAction('edit');
            ctrl.onFireAction('export');

            expect(ctrl.projectActionHandlerCallback).toHaveBeenCalledTimes(3);
        });
    });

    describe('onSelectRow(): ', function () {
        it('should call $state.go() method', function () {
            spyOn($state, 'go');

            ctrl.onSelectRow(new MouseEvent('click'));

            expect($state.go).toHaveBeenCalled();
        });
    });
});

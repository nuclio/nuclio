/*
Copyright 2023 The Nuclio Authors.

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

describe('nclEditProjectDialog component:', function () {
    var $componentController;
    var $q;
    var $rootScope;
    var ctrl;
    var scope;
    var project;

    beforeEach(function () {
        module('nuclio.app');

        inject(function (_$componentController_, _$q_, _$rootScope_) {
            $componentController = _$componentController_;
            $q = _$q_;
            $rootScope = _$rootScope_;
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
                description: 'Some description'
            }
        };
        var bindings = {
            project: project,
            confirm: angular.noop,
            closeDialog: angular.noop,
            updateProjectCallback: angular.noop
        };

        ctrl = $componentController('nclEditProjectDialog', {$scope: scope}, bindings);
        ctrl.$onInit();
    });

    afterEach(function () {
        $componentController = null;
        $q = null;
        $rootScope = null;
        ctrl = null;
    });

    describe('$onInit(): ', function () {
        it('should set copy of `ctrl.project` to `ctrl.data`', function () {
            expect(ctrl.data).toEqual(project);
        });
    });

    describe('saveProject(): ', function () {
        it('should resolve `ctrl.updateProjectCallback()` method if form is valid', function () {
            spyOn(ctrl, 'updateProjectCallback').and.returnValue($q.when());
            spyOn(ctrl, 'closeDialog').and.callThrough();

            ctrl.saveProject();
            $rootScope.$digest();

            expect(ctrl.updateProjectCallback).toHaveBeenCalledWith({ project: ctrl.data });
            expect(ctrl.closeDialog).toHaveBeenCalled();
            expect(ctrl.serverError).toBe('');
        });

        // todo error status cases
        it('should reject `ctrl.updateProjectCallback()` method if there is error ' +
            '(missing mandatory fields) is response', function () {
            spyOn(ctrl, 'updateProjectCallback').and.returnValue($q.reject({
                status: 400
            }));

            ctrl.saveProject();
            $rootScope.$digest();

            expect(ctrl.updateProjectCallback).toHaveBeenCalledWith({ project: ctrl.data });
            expect(ctrl.serverError).toBe('ERROR_MSG.UPDATE_PROJECT');
        });
    });

    describe('inputValueCallback(): ', function () {
        it('should set new value from input to `name` field', function () {
            var expectedName = 'new name';

            ctrl.inputValueCallback(expectedName, 'metadata.name');

            expect(ctrl.data.metadata.name).toBe(expectedName);
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

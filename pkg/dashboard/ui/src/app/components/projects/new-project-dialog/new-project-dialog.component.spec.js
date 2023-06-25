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

describe('nclNewProjectDialog component:', function () {
    var $componentController;
    var $q;
    var $rootScope;
    var ctrl;
    var scope;

    beforeEach(function () {
        module('nuclio.app');

        inject(function (_$componentController_, _$q_, _$rootScope_) {
            $componentController = _$componentController_;
            $q = _$q_;
            $rootScope = _$rootScope_;
        });

        scope = $rootScope.$new();
        scope.newProjectForm = {
            $valid: true
        };
        var bindings = {
            closeDialog: angular.noop,
            createProjectCallback: angular.noop
        };

        ctrl = $componentController('nclNewProjectDialog', { $scope: scope }, bindings);

        ctrl.$onInit();
    });

    afterEach(function () {
        $componentController = null;
        $q = null;
        $rootScope = null;
        ctrl = null;
    });

    describe('$onInit(): ', function () {
        it('should set blank data', function () {
            var data = {
                metadata: {
                    name: ''
                },
                spec: {
                    description: ''
                }
            };

            expect(ctrl.data).toEqual(data);
        });
    });

    describe('createProject(): ', function () {
        it('should resolve `ctrl.createProjectCallback()` method if form is valid', function () {
            spyOn(ctrl, 'createProjectCallback').and.returnValue($q.when());
            spyOn(ctrl, 'closeDialog').and.callThrough();
            var newData = {
                metadata: {
                    name: 'project-1',
                    namespace: 'nuclio'
                },
                spec: {
                    description: 'project-description',
                    created_by: 'admin',
                    created_date: '2017-06-20T08:15:56.000Z'
                }
            };
            ctrl.data = newData;

            ctrl.createProject();
            $rootScope.$digest();

            expect(ctrl.createProjectCallback).toHaveBeenCalledWith({ project: newData });
            expect(ctrl.closeDialog).toHaveBeenCalled();
        });

        // todo error status cases
        it('should reject `ctrl.createProjectCallback()` method if there is error ' +
            '(missing mandatory fields) is response', function () {
            spyOn(ctrl, 'createProjectCallback').and.returnValue($q.reject({
                status: 400
            }));

            ctrl.data = {
                metadata: {
                    name: 'project-1',
                    namespace: 'nuclio'
                },
                spec: {
                    description: 'project-description',
                    created_by: 'admin',
                    created_date: '2017-06-20T08:15:56.000Z'
                }
            };

            ctrl.createProject();
            $rootScope.$digest();

            expect(ctrl.createProjectCallback).toHaveBeenCalledWith({ project: ctrl.data });
            expect(ctrl.serverError).toBe('ERROR_MSG.UNKNOWN_ERROR_RETRY_LATER');
        });
    });

    describe('inputValueCallback(): ', function () {
        it('should set new value from input to `name` field', function () {
            var expectedName = 'new name';

            ctrl.inputValueCallback(expectedName, 'metadata.name');

            expect(ctrl.data.metadata.name).toBe(expectedName);
        });

        it('should set new value from input to the `description` field', function () {
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

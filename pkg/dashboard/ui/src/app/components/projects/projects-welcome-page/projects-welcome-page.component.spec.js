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
describe('nclProjectsWelcomePage component: ', function () {
    var $componentController;
    var $rootScope;
    var $q;
    var ngDialog;
    var ctrl;

    beforeEach(function () {
        module('nuclio.app');

        inject(function (_$rootScope_, _$componentController_, _$q_, _ngDialog_) {
            $rootScope = _$rootScope_;
            $componentController = _$componentController_;
            $q = _$q_;
            ngDialog = _ngDialog_;

            var element = angular.element('<ncl-projects-welcome-page></ncl-projects-welcome-page>');
            var ImportService = {
                importService: null
            };

            ctrl = $componentController('nclProjectsWelcomePage', {$element: element, ImportService: ImportService});
        });
    });

    afterEach(function () {
        $componentController = null;
        $rootScope = null;
        $q = null;
        ngDialog = null;
        ctrl = null;
    });

    describe('$onDestroy(): ', function () {
        it('should close opened ngDialog', function () {
            spyOn(ngDialog, 'close');

            ctrl.$onDestroy();

            expect(ngDialog.close).toHaveBeenCalled();
        });
    });

    describe('openNewProjectDialog(): ', function () {

        // todo
        it('should open ngDialog', function () {
            spyOn(ngDialog, 'open').and.callThrough();

            ctrl.openNewProjectDialog();

            expect(ngDialog.open).toHaveBeenCalled();
        });
    });
});

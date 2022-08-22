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
(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('createFunctionDataWrapper', {
            templateUrl: 'data-wrappers/create-function-data-wrapper/create-function-data-wrapper.tpl.html',
            controller: CreateFunctionDataWrapperController
        });

    function CreateFunctionDataWrapperController($scope, lodash, ngDialog, YAML, NuclioProjectsDataService, NuclioFunctionsDataService) {
        var ctrl = this;

        ctrl.templates = {};

        ctrl.createProject = createProject;
        ctrl.getFunction = getFunction;
        ctrl.getProject = getProject;
        ctrl.getProjects = getProjects;
        ctrl.getTemplates = getTemplates;
        ctrl.renderTemplate = renderTemplate;

        //
        // Public methods
        //

        /**
         * Create a single project
         * @returns {Promise}
         */
        function createProject() {
            return ngDialog.open({
                template: '<ncl-new-project-dialog data-close-dialog="closeThisDialog(project)">' +
                    '</ncl-new-project-dialog>',
                plain: true,
                scope: $scope,
                className: 'ngdialog-theme-nuclio'
            })
                .closePromise;
        }

        /**
         * Gets a single function
         * @param {Object} metadata
         * @returns {Promise}
         */
        function getFunction(metadata) {
            return NuclioFunctionsDataService.getFunction(metadata);
        }

        /**
         * Gets a single project
         * @param {string} id - project ID
         * @returns {Promise}
         */
        function getProject(id) {
            return NuclioProjectsDataService.getProject(id);
        }

        /**
         * Gets a list of all projects
         * @returns {Promise}
         */
        function getProjects() {
            return NuclioProjectsDataService.getProjects();
        }

        /**
         * Gets list of function's templates
         */
        function getTemplates() {
            return NuclioFunctionsDataService.getTemplates()
                .then(function (templates) {
                    lodash.forIn(templates, function (value) {
                        if (!lodash.has(value, 'rendered') && lodash.has(value, 'template')) {
                            var template = YAML.parse(value.template.replace(/{{\s.+\s}}/g, '"$&"'));

                            lodash.set(value, 'rendered.spec', template.spec);
                        }

                        if (!lodash.has(value, 'rendered.metadata.name')) {
                            lodash.set(value, 'rendered.metadata.name', lodash.get(value, 'metadata.name'));
                        }

                        var title = value.rendered.metadata.name.split(':')[0] + ' (' + value.rendered.spec.runtime + ')';

                        ctrl.templates[title] = value;
                    });

                    return ctrl.templates;
                });
        }

        /**
         * Render template data
         * @param {string} template
         * @returns {Promise}
         */
        function renderTemplate(template) {
            return NuclioFunctionsDataService.renderTemplate(template);
        }
    }
}());

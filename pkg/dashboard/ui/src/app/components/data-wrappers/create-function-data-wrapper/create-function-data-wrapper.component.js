(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('createFunctionDataWrapper', {
            templateUrl: 'data-wrappers/create-function-data-wrapper/create-function-data-wrapper.tpl.html',
            controller: CreateFunctionDataWrapperController
        });

    function CreateFunctionDataWrapperController(lodash, NuclioProjectsDataService, NuclioFunctionsDataService) {
        var ctrl = this;

        ctrl.templates = {};

        ctrl.createProject = createProject;
        ctrl.getProject = getProject;
        ctrl.getProjects = getProjects;
        ctrl.getTemplates = getTemplates;

        //
        // Public methods
        //

        /**
         * Create a single project
         * @param {Object} project
         * @returns {Promise}
         */
        function createProject(project) {
            return NuclioProjectsDataService.createProject(project);
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
                        var title = value.metadata.name.split(':')[0] + ' (' + value.spec.runtime + ')';

                        ctrl.templates[title] = value;
                    });

                    return ctrl.templates;
                })
        }
    }
}());

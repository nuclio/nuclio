(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('projectsWelcomePageDataWrapper', {
            bindings: {
                version: '<'
            },
            templateUrl: 'data-wrappers/projects-welcome-page-data-wrapper/projects-welcome-page-data-wrapper.tpl.html',
            controller: ProjectsWelcomePageDataWrapperController
        });

    function ProjectsWelcomePageDataWrapperController(NuclioProjectsDataService) {
        var ctrl = this;

        ctrl.createProject = createProject;

        //
        // Public methods
        //

        /**
         * Creates a new project on beck-end
         * @param {Object} projectData - project to create
         * @returns {Promise}
         */
        function createProject(projectData) {
            return NuclioProjectsDataService.createProject(projectData);
        }
    }
}());

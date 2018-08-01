(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('projectsDataWrapper', {
            templateUrl: 'data-wrappers/projects-data-wrapper/projects-data-wrapper.tpl.html',
            controller: ProjectsDataWrapperController
        });

    function ProjectsDataWrapperController($state, lodash, DialogsService, NuclioProjectsDataService) {
        var ctrl = this;

        ctrl.projects = [];

        ctrl.$onInit = onInit;

        ctrl.createProject = createProject;
        ctrl.deleteProject = deleteProject;
        ctrl.updateProject = updateProject;
        ctrl.getProjects = getProjects;

        //
        // Hook methods
        //

        /**
         * Initialization function
         */
        function onInit() {
            ctrl.namespace = lodash.get($state, 'params.namespace');
        }

        //
        // Public methods
        //

        /**
         * Creates a new project on beck-end
         * @param {Object} project - project to create
         * @returns {Promise}
         */
        function createProject(project) {
            return NuclioProjectsDataService.createProject(project);
        }

        /**
         * Deletes a project
         * @param {Object} project - project to delete
         * @returns {Promise}
         */
        function deleteProject(project) {
            return NuclioProjectsDataService.deleteProject(project);
        }

        /**
         * Updates a single project on beck-end
         */
        function updateProject(project) {
            return NuclioProjectsDataService.updateProject(project);
        }

        /**
         * Gets a list of all projects
         * @returns {Promise}
         */
        function getProjects() {
            return NuclioProjectsDataService.getProjects()
                .then(function (response) {
                    ctrl.projects = lodash.filter(response, function (projectFromResponse) {
                        var foundProject = lodash.find(ctrl.projects, ['metadata.name', projectFromResponse.metadata.name]);
                        var ui = lodash.get(foundProject, 'ui');
                        projectFromResponse.ui = lodash.defaultTo(ui, projectFromResponse.ui);

                        return lodash.isNil(ctrl.namespace) || ctrl.namespace === lodash.get(projectFromResponse, 'metadata.namespace');
                    });

                    if (lodash.isEmpty(ctrl.projects)) {
                        $state.go('app.nuclio-welcome');
                    }
                })
                .catch(function (error) {
                    DialogsService.alert('Oops: Unknown error occurred while retrieving projects');
                });
        }
    }
}());

(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('projectsFunctionsDataWrapper', {
            templateUrl: 'data-wrappers/projects-functions-data-wrapper/projects-functions-data-wrapper.tpl.html',
            controller: ProjectsFunctionsDataWrapperController
        });

    function ProjectsFunctionsDataWrapperController($q, $state, $i18next, i18next, lodash, DialogsService,
                                           NuclioProjectsDataService, NuclioFunctionsDataService) {
        var ctrl = this;

        ctrl.projects = [];

        ctrl.createFunction = createFunction;
        ctrl.createProject = createProject;
        ctrl.deleteFunction = deleteFunction;
        ctrl.deleteProject = deleteProject;
        ctrl.getFunction = getFunction;
        ctrl.getFunctions = getFunctions;
        ctrl.getProject = getProject;
        ctrl.getProjects = getProjects;
        ctrl.getStatistics = getStatistics;
        ctrl.updateFunction = updateFunction;
        ctrl.updateProject = updateProject;

        //
        // Public methods
        //

        /**
         * Deploys version
         * @param {Object} version
         * @param {string} projectID
         * @returns {Promise}
         */
        function createFunction(version, projectID) {
            return NuclioFunctionsDataService.createFunction(version, projectID);
        }

        /**
         * Creates a new project on beck-end
         * @param {Object} project - project to create
         * @returns {Promise}
         */
        function createProject(project) {
            return NuclioProjectsDataService.createProject(project);
        }

        /**
         * Deletes function
         * @param {Object} functionToDelete
         * @returns {Promise}
         */
        function deleteFunction(functionToDelete) {
            return NuclioFunctionsDataService.deleteFunction(functionToDelete);
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
         * Gets a function
         * @param {Object} metadata
         * @returns {Promise}
         */
        function getFunction(metadata) {
            return NuclioFunctionsDataService.getFunction(metadata);
        }

        /**
         * Gets functions list
         * @param {string} id - project's id
         * @returns {Promise}
         */
        function getFunctions(id) {
            return NuclioFunctionsDataService.getFunctions(id);
        }

        /**
         * Gets a list of all project
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
            return NuclioProjectsDataService.getProjects()
                .then(function (projectsFromResponse) {
                    ctrl.projects = lodash.map(projectsFromResponse, function (projectFromResponse) {
                        var foundProject = lodash.find(ctrl.projects, ['metadata.name', projectFromResponse.metadata.name]);
                        var ui = lodash.get(foundProject, 'ui');
                        projectFromResponse.ui = lodash.defaultTo(ui, projectFromResponse.ui);

                        return projectFromResponse;
                    });

                    if (lodash.isEmpty(ctrl.projects)) {
                        $state.go('app.nuclio-welcome');
                    }
                })
                .catch(function (error) {
                    DialogsService.alert($i18next.t('functions:ERROR_MSG.GET_PROJECTS', {lng: i18next.language}));
                });
        }

        /**
         * Gets statistics
         * @returns {Promise}
         */
        function getStatistics() {
            return $q.reject({msg: 'N/A'});
        }

        /**
         * Updates function
         * @param functionData
         * @param projectID
         * @returns {*|Promise}
         */
        function updateFunction(functionData, projectID) {
            return NuclioFunctionsDataService.updateFunction(functionData, projectID);
        }

        /**
         * Updates a single project on beck-end
         */
        function updateProject(project) {
            return NuclioProjectsDataService.updateProject(project);
        }


    }
}());

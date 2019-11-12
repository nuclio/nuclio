(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('versionDataWrapper', {
            bindings: {
                version: '<'
            },
            templateUrl: 'data-wrappers/version-data-wrapper/version-data-wrapper.tpl.html',
            controller: VersionDataWrapperController
        });

    function VersionDataWrapperController(NuclioFunctionsDataService, NuclioProjectsDataService) {
        var ctrl = this;

        ctrl.createFunction = createFunction;
        ctrl.deleteFunction = deleteFunction;
        ctrl.getFunction = getFunction;
        ctrl.getFunctions = getFunctions;
        ctrl.getProject = getProject;
        ctrl.updateFunction = updateFunction;

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
         * Deletes function
         * @param {Object} functionToDelete
         * @returns {Promise}
         */
        function deleteFunction(functionToDelete) {
            return NuclioFunctionsDataService.deleteFunction(functionToDelete);
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
         * @param {string} id
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
         * Deploys version
         * @param {Object} version
         * @param {string} projectID
         * @returns {Promise}
         */
        function updateFunction(version, projectID) {
            return NuclioFunctionsDataService.updateFunction(version, projectID);
        }
    }
}());

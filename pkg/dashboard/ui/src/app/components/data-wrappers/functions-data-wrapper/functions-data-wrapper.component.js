(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('functionsDataWrapper', {
            templateUrl: 'data-wrappers/functions-data-wrapper/functions-data-wrapper.tpl.html',
            controller: FunctionsDataWrapperController
        });

    function FunctionsDataWrapperController($q, NuclioProjectsDataService, NuclioFunctionsDataService) {
        var ctrl = this;

        ctrl.createFunction = createFunction;
        ctrl.getExternalIPAddresses = getExternalIPAddresses;
        ctrl.getProject = getProject;
        ctrl.getFunction = getFunction;
        ctrl.getFunctions = getFunctions;
        ctrl.getStatistics = getStatistics;
        ctrl.deleteFunction = deleteFunction;
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
         * Gets external IP addresses
         * @returns {Promise}
         */
        function getExternalIPAddresses() {
            return NuclioProjectsDataService.getExternalIPAddresses();
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
         * Gets statistics
         * @returns {Promise}
         */
        function getStatistics() {
            return $q.reject({msg: 'N/A'});
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
         * Updates function
         * @param functionData
         * @param projectID
         * @returns {*|Promise}
         */
        function updateFunction(functionData, projectID) {
            return NuclioFunctionsDataService.updateFunction(functionData, projectID);
        }
    }
}());

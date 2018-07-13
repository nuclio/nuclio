(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('functionsDataWrapper', {
            templateUrl: 'data-wrappers/functions-data-wrapper/functions-data-wrapper.tpl.html',
            controller: FunctionsDataWrapperController
        });

    function FunctionsDataWrapperController(NuclioProjectsDataService, NuclioFunctionsDataService) {
        var ctrl = this;

        ctrl.getExternalIPAddresses = getExternalIPAddresses;
        ctrl.getProject = getProject;
        ctrl.getFunctions = getFunctions;
        ctrl.deleteFunction = deleteFunction;

        //
        // Public methods
        //

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
         * Gets functions list
         * @param {string} id
         * @param {string} namespace
         * @returns {Promise}
         */
        function getFunctions(id, namespace) {
            return NuclioFunctionsDataService.getFunctions(namespace, id);
        }

        /**
         * Deletes function
         * @param {Object} functionToDelete
         * @returns {Promise}
         */
        function deleteFunction(functionToDelete) {
            return NuclioFunctionsDataService.deleteFunction(functionToDelete);
        }
    }
}());

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

        ctrl.deployVersion = deployVersion;
        ctrl.deleteFunction = deleteFunction;
        ctrl.getExternalIPAddresses = getExternalIPAddresses;
        ctrl.getProject = getProject;
        ctrl.getFunction = getFunction;

        //
        // Public methods
        //

        /**
         * Deploys version
         * @param {Object} version
         * @param {string} projectID
         * @returns {Promise}
         */
        function deployVersion(version, projectID) {
            return NuclioFunctionsDataService.updateFunction(version, projectID);
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
    }
}());

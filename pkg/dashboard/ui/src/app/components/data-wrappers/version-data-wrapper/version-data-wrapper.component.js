(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('versionDataWrapper', {
            bindings: {
                project: '<',
                version: '<'
            },
            templateUrl: 'data-wrappers/version-data-wrapper/version-data-wrapper.tpl.html',
            controller: VersionDataWrapperController
        });

    function VersionDataWrapperController(NuclioFunctionsDataService) {
        var ctrl = this;

        ctrl.createFunction = createFunction;
        ctrl.deleteFunction = deleteFunction;
        ctrl.getFunction = getFunction;
        ctrl.getFunctions = getFunctions;
        ctrl.updateFunction = updateFunction;

        //
        // Public methods
        //

        /**
         * Deploys version
         * @param {Object} version
         * @param {string} projectId
         * @returns {Promise}
         */
        function createFunction(version, projectId) {
            return NuclioFunctionsDataService.createFunction(version, projectId);
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
         * @param {boolean} enrichApiGateways
         * @returns {Promise}
         */
        function getFunction(metadata, enrichApiGateways) {
            return NuclioFunctionsDataService.getFunction(metadata, enrichApiGateways);
        }

        /**
         * Gets functions list
         * @param {string} id
         * @param {boolean} enrichApiGateways
         * @returns {Promise}
         */
        function getFunctions(id, enrichApiGateways) {
            return NuclioFunctionsDataService.getFunctions(id, enrichApiGateways);
        }

        /**
         * Deploys version
         * @param {Object} version
         * @param {string} projectId
         * @returns {Promise}
         */
        function updateFunction(version, projectId) {
            return NuclioFunctionsDataService.updateFunction(version, projectId);
        }
    }
}());

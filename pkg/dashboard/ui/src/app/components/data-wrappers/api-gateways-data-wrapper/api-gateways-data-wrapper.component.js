(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('apiGatewaysDataWrapper', {
            bindings: {
                project: '<'
            },
            templateUrl: 'data-wrappers/api-gateways-data-wrapper/api-gateways-data-wrapper.tpl.html',
            controller: ApiGatewaysDataWrapperController
        });

    function ApiGatewaysDataWrapperController(NuclioApiGatewaysDataService, NuclioFunctionsDataService) {
        var ctrl = this;

        ctrl.createApiGateway = createApiGateway;
        ctrl.deleteApiGateway = deleteApiGateway;
        ctrl.getApiGateway = getApiGateway;
        ctrl.getApiGateways = getApiGateways;
        ctrl.getFunctions = getFunctions;
        ctrl.updateApiGateway = updateApiGateway;

        //
        // Public methods
        //

        /**
         * Creates the new Api Gateway
         * @param {Object} apiGateway
         * @param {string} projectName
         * @returns {Promise}
         */
        function createApiGateway(apiGateway, projectName) {
            return NuclioApiGatewaysDataService.createApiGateway(apiGateway, projectName);
        }

        /**
         * Deletes Api Gateway
         * @param {Object} apiGateway
         * @returns {Promise}
         */
        function deleteApiGateway(apiGateway) {
            return NuclioApiGatewaysDataService.deleteApiGateway(apiGateway);
        }

        /**
         * Gets Api Gateway
         * @param {string} projectName
         * @param {string} apiGatewayName
         * @returns {Promise}
         */
        function getApiGateway(projectName, apiGatewayName) {
            return NuclioApiGatewaysDataService.getApiGateway(projectName, apiGatewayName);
        }

        /**
         * Gets Api Gateways list
         * @param {string} projectName
         * @returns {Promise}
         */
        function getApiGateways(projectName) {
            return NuclioApiGatewaysDataService.getApiGateways(projectName);
        }

        /**
         * Gets functions list
         * @param {string} projectName
         * @param {boolean} enrichApiGateways
         * @returns {Promise}
         */
        function getFunctions(projectName, enrichApiGateways) {
            return NuclioFunctionsDataService.getFunctions(projectName, enrichApiGateways);
        }

        /**
         * Updates the Api Gateway
         * @param {Object} apiGateway
         * @param {string} projectName
         * @returns {Promise}
         */
        function updateApiGateway(apiGateway, projectName) {
            return NuclioApiGatewaysDataService.updateApiGateway(apiGateway, projectName);
        }
    }
}());

/*
Copyright 2023 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

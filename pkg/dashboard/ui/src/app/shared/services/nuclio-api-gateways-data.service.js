/*
Copyright 2017 The Nuclio Authors.

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
        .factory('NuclioApiGatewaysDataService', NuclioApiGatewaysDataService);

    function NuclioApiGatewaysDataService(lodash, NuclioClientService, NuclioNamespacesDataService) {
        return {
            createApiGateway: createApiGateway,
            deleteApiGateway: deleteApiGateway,
            getApiGateway: getApiGateway,
            getApiGateways: getApiGateways,
            updateApiGateway: updateApiGateway
        };

        /**
         * Creates a new API Gateway
         * @param {Object} apiGateway - the API Gateway object
         * @param {string} projectName - the name of the project
         * @returns {Promise}
         */
        function createApiGateway(apiGateway, projectName) {
            var headers = {
                'Content-Type': 'application/json',
                'x-nuclio-project-name': projectName,
                'x-nuclio-agw-validate-functions-existence': true
            };
            var namespace = NuclioNamespacesDataService.getNamespace();

            if (!lodash.isNil(namespace)) {
                lodash.set(apiGateway, 'metadata.namespace', namespace);
            }

            var config = {
                method: 'post',
                url: NuclioClientService.buildUrlWithPath('api_gateways'),
                headers: headers,
                data: apiGateway,
                withCredentials: false
            };

            return NuclioClientService.makeRequest(config);
        }

        /**
         * Deletes an API Gateway
         * @param {Object} apiGateway
         * @returns {Promise}
         */
        function deleteApiGateway(apiGateway) {
            var headers = {
                'Content-Type': 'application/json'
            };

            var config = {
                method: 'delete',
                url: NuclioClientService.buildUrlWithPath('api_gateways'),
                headers: headers,
                data: apiGateway,
                withCredentials: false
            };

            return NuclioClientService.makeRequest(config);
        }

        /**
         * Gets API Gateway
         * @param {string} projectName - the name of the project
         * @param {string} apiGatewayName - the name of the API Gateways
         * @returns {Promise}
         */
        function getApiGateway(projectName, apiGatewayName) {
            var headers = {
                'Content-Type': 'application/json',
                'x-nuclio-project-name': projectName
            };

            lodash.assign(headers, NuclioNamespacesDataService.getNamespaceHeader('x-nuclio-api-gateway-namespace'));

            var config = {
                method: 'get',
                headers: headers,
                withCredentials: false,
                url: NuclioClientService.buildUrlWithPath('api_gateways/') + apiGatewayName
            };

            return NuclioClientService.makeRequest(config);
        }

        /**
         * Gets API Gateways
         * @param {string} projectName - the name of the project
         * @returns {Promise}
         */
        function getApiGateways(projectName) {
            var headers = {
                'x-nuclio-project-name': projectName
            };

            lodash.assign(headers, NuclioNamespacesDataService.getNamespaceHeader('x-nuclio-api-gateway-namespace'));

            var config = {
                method: 'get',
                url: NuclioClientService.buildUrlWithPath('api_gateways'),
                headers: headers,
                withCredentials: false
            };

            return NuclioClientService.makeRequest(config, true, false, false);
        }

        /**
         * Updates an API Gateway
         * @param {Object} apiGateway - the API Gateway object
         * @param {string} projectName - the name of the project
         * @returns {Promise}
         */
        function updateApiGateway(apiGateway, projectName) {
            var headers = {
                'Content-Type': 'application/json',
                'x-nuclio-project-name': projectName,
                'x-nuclio-agw-validate-functions-existence': true
            };
            var namespace = NuclioNamespacesDataService.getNamespace();

            if (!lodash.isNil(namespace)) {
                lodash.set(apiGateway, 'metadata.namespace', namespace);
            }

            var config = {
                method: 'put',
                url: NuclioClientService.buildUrlWithPath('api_gateways'),
                headers: headers,
                data: apiGateway,
                withCredentials: false
            };

            return NuclioClientService.makeRequest(config);
        }
    }
}());

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
                'x-nuclio-project-name': projectName
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
         * @param {string} apiGatewayName - the name of the API Gateways
         * @returns {Promise}
         */
        function getApiGateway(apiGatewayName) {
            var headers = {
                'Content-Type': 'application/json',
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
                'x-nuclio-project-name': projectName
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

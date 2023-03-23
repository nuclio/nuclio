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
        .factory('NuclioFunctionsDataService', NuclioFunctionsDataService);

    function NuclioFunctionsDataService(lodash, NuclioClientService, NuclioNamespacesDataService) {
        return {
            createFunction: createFunction,
            deleteFunction: deleteFunction,
            getFunction: getFunction,
            getFunctions: getFunctions,
            getTemplates: getTemplates,
            renderTemplate: renderTemplate,
            updateFunction: updateFunction
        };

        //
        // Public methods
        //

        /**
         * Update existing function with new data
         * @param {Object} functionDetails
         * @param {string} projectName - the name of the project containing the function
         * @param {boolean} [importProcess] - `true` if importing process
         * @param {boolean} withTimeoutHeader - `true` if additional header should be added
         * @returns {Promise}
         */
        function createFunction(functionDetails, projectName, importProcess, withTimeoutHeader) {
            var headers = {
                'Content-Type': 'application/json',
                'x-nuclio-project-name': projectName
            };

            if (withTimeoutHeader) {
                lodash.set(headers, 'x-nuclio-creation-state-updated-timeout', '5m');
            }

            lodash.assign(headers, NuclioNamespacesDataService.getNamespaceHeader('x-nuclio-function-namespace'));

            var namespace = NuclioNamespacesDataService.getNamespace();
            if (!lodash.isNil(namespace)) {
                lodash.set(functionDetails, 'metadata.namespace', namespace);
            }

            var config = {
                method: 'post',
                url: NuclioClientService.buildUrlWithPath('functions'),
                params: {
                    import: importProcess
                },
                headers: headers,
                data: functionDetails,
                withCredentials: false
            };

            return NuclioClientService.makeRequest(config);
        }

        /**
         * Gets function details
         * @param {Object} functionData
         * @param {boolean} enrichApiGateways - determines whether to enrich functions with their related API gateways
         * @returns {Promise}
         */
        function getFunction(functionData, enrichApiGateways) {
            var headers = {
                'Content-Type': 'application/json',
                'x-nuclio-project-name': functionData.projectName,
                'x-nuclio-function-enrich-apigateways': enrichApiGateways
            };

            lodash.assign(headers, NuclioNamespacesDataService.getNamespaceHeader('x-nuclio-function-namespace'));

            var config = {
                method: 'get',
                headers: headers,
                withCredentials: false,
                url: NuclioClientService.buildUrlWithPath('functions/') + functionData.name
            };

            return NuclioClientService.makeRequest(config);
        }

        /**
         * Deletes function
         * @param {Object} functionData
         * @param {Boolean} ignoreValidation - determines whether to forcibly remove the function
         * @returns {Promise}
         */
        function deleteFunction(functionData, ignoreValidation) {
            var headers = {
                'Content-Type': 'application/json'
            };
            var namespace = NuclioNamespacesDataService.getNamespace();

            if (lodash.isNil(namespace)) {
                functionData = lodash.omit(functionData, 'namespace')
            }

            if (ignoreValidation) {
                headers['x-nuclio-delete-function-ignore-state-validation'] = true;
            }

            var config = {
                method: 'delete',
                url: NuclioClientService.buildUrlWithPath('functions'),
                headers: headers,
                data: {
                    'metadata': functionData
                },
                withCredentials: false
            };

            return NuclioClientService.makeRequest(config);
        }

        /**
         * Gets functions list
         * @param {string} projectName - the name of the project containing the function
         * @param {boolean} enrichApiGateways - determines whether to enrich functions with their related API gateways
         * @returns {Promise}
         */
        function getFunctions(projectName, enrichApiGateways) {
            var headers = {
                'x-nuclio-project-name': projectName,
                'x-nuclio-function-enrich-apigateways': enrichApiGateways
            };

            lodash.assign(headers, NuclioNamespacesDataService.getNamespaceHeader('x-nuclio-function-namespace'));

            var config = {
                method: 'get',
                url: NuclioClientService.buildUrlWithPath('functions'),
                headers: headers,
                withCredentials: false
            };

            return NuclioClientService.makeRequest(config, true, false, false);
        }

        /**
         * Update existing function with new data
         * @param {Object} functionDetails
         * @param {string} projectName - the name of the project containing the function
         * @param {boolean} withTimeoutHeader - `true` if additional header should be added
         * @returns {Promise}
         */
        function updateFunction(functionDetails, projectName, withTimeoutHeader) {
            var headers = {
                'Content-Type': 'application/json',
                'x-nuclio-project-name': projectName
            };

            if (withTimeoutHeader) {
                lodash.set(headers, 'x-nuclio-creation-state-updated-timeout', '5m');
            }

            lodash.assign(headers, NuclioNamespacesDataService.getNamespaceHeader('x-nuclio-function-namespace'));

            var namespace = NuclioNamespacesDataService.getNamespace();
            if (!lodash.isNil(namespace)) {
                lodash.set(functionDetails, 'metadata.namespace', namespace);
            }

            var functionName = lodash.get(functionDetails, 'metadata.name');

            var config = {
                method: 'put',
                url: NuclioClientService.buildUrlWithPath('functions/' + functionName),
                headers: headers,
                data: functionDetails,
                withCredentials: false
            };

            return NuclioClientService.makeRequest(config);
        }

        /**
         * Gets templates for function
         * @returns {Promise}
         */
        function getTemplates() {
            var config = {
                method: 'get',
                withCredentials: false,
                url: NuclioClientService.buildUrlWithPath('function_templates')
            };

            return NuclioClientService.makeRequest(config);
        }

        /**
         * Render template data
         * @param {string} template
         * @returns {Promise}
         */
        function renderTemplate(template) {
            var config = {
                method: 'post',
                withCredentials: false,
                data: template,
                url: NuclioClientService.buildUrlWithPath('function_templates/render')
            };

            return NuclioClientService.makeRequest(config);
        }
    }
}());

(function () {
    'use strict';

    angular.module('iguazio.app')
        .factory('NuclioFunctionsDataService', NuclioFunctionsDataService);

    function NuclioFunctionsDataService(NuclioClientService) {
        return {
            createFunction: createFunction,
            deleteFunction: deleteFunction,
            initVersionActions: initVersionActions,
            getFunction: getFunction,
            getFunctions: getFunctions,
            getTemplates: getTemplates,
            updateFunction: updateFunction
        };

        //
        // Public methods
        //

        /**
         * Gets function details
         * @param {Object} functionData
         * @returns {Promise}
         */
        function createFunction(functionData) {
            var headers = {
                'Content-Type': 'application/json',
                'x-nuclio-function-namespace': functionData.namespace
            };

            var config = {
                data: functionData,
                method: 'post',
                headers: headers,
                withCredentials: false,
                url: NuclioClientService.buildUrlWithPath('functions')
            };

            return NuclioClientService.makeRequest(config);
        }

        /**
         * Gets function details
         * @param {Object} functionData
         * @returns {Promise}
         */
        function getFunction(functionData) {
            var headers = {
                'Content-Type': 'application/json',
                'x-nuclio-function-namespace': functionData.namespace
            };
            var config = {
                method: 'get',
                headers: headers,
                withCredentials: false,
                url: NuclioClientService.buildUrlWithPath('functions/') + functionData.name
            };

            return NuclioClientService.makeRequest(config).then(function (response) {
                return response.data;
            });
        }

        /**
         * Gets function details
         * @param {Object} functionData
         * @returns {Promise}
         */
        function deleteFunction(functionData) {
            var headers = {
                'Content-Type': 'application/json'
            };
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
         * Actions for Action panel
         * @returns {Object[]} - array of actions
         */
        function initVersionActions() {
            var actions = [
                {
                    label: 'Edit',
                    id: 'edit',
                    icon: 'igz-icon-edit',
                    active: true,
                    capability: 'nuclio.functions.versions.edit'
                },
                {
                    label: 'Delete',
                    id: 'delete',
                    icon: 'igz-icon-trash',
                    active: true,
                    capability: 'nuclio.functions.versions.delete',
                    confirm: {
                        message: 'Are you sure you want to delete selected version?',
                        yesLabel: 'Yes, Delete',
                        noLabel: 'Cancel',
                        type: 'critical_alert'
                    }
                }
            ];

            return actions;
        }

        /**
         * Gets functions list
         * @param {string} namespace
         * @returns {Promise}
         */
        function getFunctions(namespace) {
            var headers = {
                'x-nuclio-function-namespace': namespace
            };
            var config = {
                method: 'get',
                url: NuclioClientService.buildUrlWithPath('functions'),
                headers: headers,
                withCredentials: false
            };

            return NuclioClientService.makeRequest(config, false, false);
        }

        /**
         * Update existing function with new data
         * @param {Object} functionDetails
         * @returns {Promise}
         */
        function updateFunction(functionDetails) {
            var headers = {
                'Content-Type': 'application/json'
            };
            var config = {
                method: 'post',
                url: NuclioClientService.buildUrlWithPath('functions'),
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
    }
}());

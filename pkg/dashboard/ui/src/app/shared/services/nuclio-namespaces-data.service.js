(function () {
    'use strict';

    angular.module('nuclio.app')
        .factory('NuclioNamespacesDataService', NuclioNamespacesDataService);

    function NuclioNamespacesDataService(NuclioClientService, DialogsService, lodash) {

        var service = {
            getNamespaces: getNamespaces,
            getNamespace: getNamespace,
            getNamespaceHeader: getNamespaceHeader,
            initNamespaceData: initNamespaceData,
            namespaceData: {
                namespaces: [],
                namespacesExist: false,
                selectedNamespace: null
            }
        };

        return service;

        //
        // Public methods
        //

        /**
         * Gets all namespaces
         * @returns {Promise}
         */
        function getNamespaces() {
            return NuclioClientService.makeRequest(
                {
                    method: 'GET',
                    url: NuclioClientService.buildUrlWithPath('namespaces', ''),
                    withCredentials: false
                })
                .then(function (response) {
                    return response.data;
                })
        }

        /**
         * Gets all namespace
         * @returns {string}
         */
        function getNamespace() {
            var namespace = localStorage.getItem('namespace');
            return !lodash.isNil(namespace) && namespace !== '' ? namespace : null;
        }

        /**
         * Gets namespace header
         * @param {string} headerTitle - title of namespace header
         * @returns {Object}
         */
        function getNamespaceHeader(headerTitle) {
            var namespace = service.getNamespace();
            var headerObj = {};

            if (!lodash.isNil(namespace)) {
                headerObj[headerTitle] = namespace;
            }

            return headerObj;
        }

        /**
         * Init namespace data
         * @returns {Promise}
         */
        function initNamespaceData() {
            return service.getNamespaces()
                .then(function (response) {
                    if (lodash.isEmpty(response.namespaces.names)) {
                        localStorage.removeItem('namespace');
                    } else {
                        var namespacesExist = true;
                        var namespaces = lodash.map(response.namespaces.names, function (name) {
                            return {
                                type: 'namespace',
                                id: name,
                                name: name
                            };
                        });
                        var namespaceFromLocalStorage = localStorage.getItem('namespace');

                        var selectedNamespace = lodash.find(namespaces, { name: namespaceFromLocalStorage });
                        if (lodash.isNil(selectedNamespace)) {
                            selectedNamespace = namespaces[0];
                            localStorage.setItem('namespace', selectedNamespace.name);
                        }

                        service.namespaceData = {
                            namespaces: namespaces,
                            namespacesExist: namespacesExist,
                            selectedNamespace: selectedNamespace
                        }
                    }

                    return service.namespaceData;
                })
                .catch(function () {
                    localStorage.removeItem('namespace');

                    DialogsService.alert('Oops: Unknown error occurred while retrieving namespaces');
                });
        }
    }
}());

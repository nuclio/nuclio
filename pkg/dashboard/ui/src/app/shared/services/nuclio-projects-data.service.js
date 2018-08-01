(function () {
    'use strict';

    angular.module('nuclio.app')
        .factory('NuclioProjectsDataService', NuclioProjectsDataService);

    function NuclioProjectsDataService(NuclioClientService) {
        return {
            createProject: createProject,
            deleteProject: deleteProject,
            getExternalIPAddresses: getExternalIPAddresses,
            getNamespaces: getNamespaces,
            getProject: getProject,
            getProjects: getProjects,
            updateProject: updateProject
        };

        //
        // Public methods
        //

        /**
         * Creates a new project
         * @param {Object} project - the project to create
         */
        function createProject(project) {
            var headers = {
                'Content-Type': 'application/json'
            };
            var data = {
                metadata: {},
                spec: project.spec
            };

            return NuclioClientService.makeRequest(
                {
                    method: 'POST',
                    url: NuclioClientService.buildUrlWithPath('projects', ''),
                    headers: headers,
                    data: data,
                    withCredentials: false
                })
                .then(function (response) {
                    return response.data;
                });
        }

        /**
         * Deletes a project
         * @param {Object} project - the project to create
         */
        function deleteProject(project) {
            var headers = {
                'Content-Type': 'application/json'
            };
            var data = {
                metadata: project.metadata
            };

            return NuclioClientService.makeRequest(
                {
                    method: 'DELETE',
                    url: NuclioClientService.buildUrlWithPath('projects', ''),
                    headers: headers,
                    data: data,
                    withCredentials: false
                })
                .then(function (response) {
                    return response.data;
                });
        }

        /**
         * Gets all projects
         * @returns {Promise}
         */
        function getProjects() {
            return NuclioClientService.makeRequest(
                {
                    method: 'GET',
                    url: NuclioClientService.buildUrlWithPath('projects', ''),
                    withCredentials: false
                })
                .then(function (response) {
                    return response.data;
                });
        }

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
                });
        }

        /**
         * Gets project by id
         * @param {string} id - id of project
         * @returns {Promise}
         */
        function getProject(id) {
            return NuclioClientService.makeRequest(
                {
                    method: 'GET',
                    url: NuclioClientService.buildUrlWithPath('projects/', id),
                    withCredentials: false
                })
                .then(function (response) {
                    return response.data;
                });
        }

        /**
         * Updates a new project
         * @param {Object} project - the project to update
         */
        function updateProject(project) {
            var headers = {
                'Content-Type': 'application/json'
            };
            var data = {
                metadata: project.metadata,
                spec: project.spec
            };

            return NuclioClientService.makeRequest(
                {
                    method: 'PUT',
                    url: NuclioClientService.buildUrlWithPath('projects', ''),
                    headers: headers,
                    data: data,
                    withCredentials: false
                })
                .then(function (response) {
                    return response.data;
                });
        }

        /**
         * Gets external IP addresses for functions
         * @returns {Promise}
         */
        function getExternalIPAddresses() {
            return NuclioClientService.makeRequest(
                {
                    method: 'GET',
                    url: NuclioClientService.buildUrlWithPath('external_ip_addresses'),
                    withCredentials: false
                })
                .then(function (response) {
                    return response.data;
                });
        }
    }
}());

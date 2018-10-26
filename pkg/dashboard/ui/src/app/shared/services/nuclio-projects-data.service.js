(function () {
    'use strict';

    angular.module('nuclio.app')
        .factory('NuclioProjectsDataService', NuclioProjectsDataService);

    function NuclioProjectsDataService(lodash, NuclioClientService, NuclioNamespacesDataService) {
        return {
            createProject: createProject,
            deleteProject: deleteProject,
            getExternalIPAddresses: getExternalIPAddresses,
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
            var namespace = NuclioNamespacesDataService.getNamespace();

            if (!lodash.isNil(namespace)) {
                data.metadata.namespace = namespace;
            }

            return NuclioClientService.makeRequest(
                {
                    method: 'POST',
                    url: NuclioClientService.buildUrlWithPath('projects', ''),
                    headers: headers,
                    data: data,
                    withCredentials: false
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
            var namespace = NuclioNamespacesDataService.getNamespace();

            if (lodash.isNil(namespace)) {
                data.metadata = lodash.omit(data.metadata, 'namespace');
            }

            return NuclioClientService.makeRequest(
                {
                    method: 'DELETE',
                    url: NuclioClientService.buildUrlWithPath('projects', ''),
                    headers: headers,
                    data: data,
                    withCredentials: false
                });
        }

        /**
         * Gets all projects
         * @returns {Promise}
         */
        function getProjects() {
            var headers = NuclioNamespacesDataService.getNamespaceHeader('x-nuclio-project-namespace');

            return NuclioClientService.makeRequest(
                {
                    headers: headers,
                    method: 'GET',
                    url: NuclioClientService.buildUrlWithPath('projects', ''),
                    withCredentials: false
                });
        }

        /**
         * Gets project by id
         * @param {string} id - id of project
         * @returns {Promise}
         */
        function getProject(id) {
            var headers = NuclioNamespacesDataService.getNamespaceHeader('x-nuclio-project-namespace');

            return NuclioClientService.makeRequest(
                {
                    headers: headers,
                    method: 'GET',
                    url: NuclioClientService.buildUrlWithPath('projects/', id),
                    withCredentials: false
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
            var namespace = NuclioNamespacesDataService.getNamespace();

            if (lodash.isNil(namespace)) {
                data.metadata = lodash.omit(data.metadata, 'namespace');
            }


            return NuclioClientService.makeRequest(
                {
                    method: 'PUT',
                    url: NuclioClientService.buildUrlWithPath('projects', ''),
                    headers: headers,
                    data: data,
                    withCredentials: false
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
                });
        }
    }
}());

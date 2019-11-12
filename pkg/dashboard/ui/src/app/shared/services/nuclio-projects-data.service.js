(function () {
    'use strict';

    angular.module('nuclio.app')
        .factory('NuclioProjectsDataService', NuclioProjectsDataService);

    function NuclioProjectsDataService(lodash, NuclioClientService, NuclioNamespacesDataService) {
        return {
            createProject: createProject,
            deleteProject: deleteProject,
            getFrontendSpec: getFrontendSpec,
            getProject: getProject,
            getProjects: getProjects,
            updateProject: updateProject
        };

        //
        // Public methods
        //

        /**
         * Creates a new project
         * @param {Promise} project - the project to create
         */
        function createProject(project) {
            var headers = {
                'Content-Type': 'application/json'
            };
            var data = lodash.pick(project, ['metadata', 'spec']);
            var namespace = NuclioNamespacesDataService.getNamespace();

            if (!lodash.isNil(namespace)) {
                data.metadata.namespace = namespace;
            }

            return NuclioClientService.makeRequest({
                method: 'POST',
                url: NuclioClientService.buildUrlWithPath('projects', ''),
                headers: headers,
                data: data,
                withCredentials: false
            });
        }

        /**
         * Deletes a project
         * @param {Promise} project - the project to create
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

            return NuclioClientService.makeRequest({
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

            return NuclioClientService.makeRequest({
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

            return NuclioClientService.makeRequest({
                headers: headers,
                method: 'GET',
                url: NuclioClientService.buildUrlWithPath('projects/', id),
                withCredentials: false
            });
        }

        /**
         * Updates a new project
         * @param {Promise} project - the project to update
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

            return NuclioClientService.makeRequest({
                method: 'PUT',
                url: NuclioClientService.buildUrlWithPath('projects', ''),
                headers: headers,
                data: data,
                withCredentials: false
            });
        }

        /**
         * Gets front-end spec.
         * @returns {Promise.<{defaultHTTPIngressHostTemplate: string, externalIPAddresses: Array.<string>,
         * namespace: string}>}
         */
        function getFrontendSpec() {
            return NuclioClientService.makeRequest({
                method: 'GET',
                url: NuclioClientService.buildUrlWithPath('frontend_spec'),
                withCredentials: false
            });
        }
    }
}());

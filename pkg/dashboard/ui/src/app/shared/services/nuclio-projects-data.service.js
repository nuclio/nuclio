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
         * @param {boolean} [importProcess] - `true` if importing process
         */
        function createProject(project, importProcess) {
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
                url: NuclioClientService.buildUrlWithPath('projects'),
                params: {
                    import: importProcess
                },
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
            if (lodash.has(project, 'ui.forceDelete')) {
                lodash.set(headers, 'x-nuclio-delete-project-strategy', 'cascading');
            }
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
                spec: lodash.omit(project.spec, 'displayName')
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
         * @returns {Promise.<Object>}
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

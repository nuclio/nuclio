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
        .component('versionDataWrapper', {
            bindings: {
                project: '<',
                version: '<'
            },
            templateUrl: 'data-wrappers/version-data-wrapper/version-data-wrapper.tpl.html',
            controller: VersionDataWrapperController
        });

    function VersionDataWrapperController(NuclioFunctionsDataService) {
        var ctrl = this;

        ctrl.createFunction = createFunction;
        ctrl.deleteFunction = deleteFunction;
        ctrl.getFunction = getFunction;
        ctrl.getFunctions = getFunctions;
        ctrl.updateFunction = updateFunction;

        //
        // Public methods
        //

        /**
         * Deploys version
         * @param {Object} version
         * @param {string} projectId
         * @returns {Promise}
         */
        function createFunction(version, projectId) {
            return NuclioFunctionsDataService.createFunction(version, projectId);
        }

        /**
         * Deletes function
         * @param {Object} functionToDelete
         * @param {Boolean} ignoreValidation - determines whether to forcibly remove the function
         * @returns {Promise}
         */
        function deleteFunction(functionToDelete, ignoreValidation) {
            return NuclioFunctionsDataService.deleteFunction(functionToDelete, ignoreValidation);
        }

        /**
         * Gets a function
         * @param {Object} metadata
         * @param {boolean} enrichApiGateways
         * @returns {Promise}
         */
        function getFunction(metadata, enrichApiGateways) {
            return NuclioFunctionsDataService.getFunction(metadata, enrichApiGateways);
        }

        /**
         * Gets functions list
         * @param {string} id
         * @param {boolean} enrichApiGateways
         * @returns {Promise}
         */
        function getFunctions(id, enrichApiGateways) {
            return NuclioFunctionsDataService.getFunctions(id, enrichApiGateways);
        }

        /**
         * Deploys version
         * @param {Object} version
         * @param {string} projectId
         * @returns {Promise}
         */
        function updateFunction(version, projectId) {
            return NuclioFunctionsDataService.updateFunction(version, projectId);
        }
    }
}());

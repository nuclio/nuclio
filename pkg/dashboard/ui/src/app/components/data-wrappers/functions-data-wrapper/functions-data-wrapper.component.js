/*
Copyright 2023 The Nuclio Authors.

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
        .component('functionsDataWrapper', {
            bindings: {
                project: '<'
            },
            templateUrl: 'data-wrappers/functions-data-wrapper/functions-data-wrapper.tpl.html',
            controller: FunctionsDataWrapperController
        });

    function FunctionsDataWrapperController($q, $i18next, i18next, NuclioFunctionsDataService) {
        var ctrl = this;
        var lng = i18next.language;

        ctrl.createFunction = createFunction;
        ctrl.getFunction = getFunction;
        ctrl.getFunctions = getFunctions;
        ctrl.getStatistics = getStatistics;
        ctrl.deleteFunction = deleteFunction;
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
         * Gets statistics
         * @returns {Promise}
         */
        function getStatistics() {
            return $q.reject({ msg: $i18next.t('common:N_A', { lng: lng }) });
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
         * Updates function
         * @param functionData
         * @param projectId
         * @returns {*|Promise}
         */
        function updateFunction(functionData, projectId) {
            return NuclioFunctionsDataService.updateFunction(functionData, projectId);
        }
    }
}());

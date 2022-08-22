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
        .component('nclBreadcrumbsDataWrapper', {
            templateUrl: 'data-wrappers/ncl-breadcrumbs-data-wrapper/ncl-breadcrumbs-data-wrapper.tpl.html',
            controller: NclBreadcrumbsDataWrapperController
        });

    function NclBreadcrumbsDataWrapperController(NuclioProjectsDataService, NuclioFunctionsDataService) {
        var ctrl = this;

        ctrl.getProjects = getProjects;
        ctrl.getFunctions = getFunctions;

        //
        // Public methods
        //

        /**
         * Gets a list of projects
         * @returns {Promise}
         */
        function getProjects() {
            return NuclioProjectsDataService.getProjects();
        }

        /**
         * Gets a list of functions
         * @param {string} id - project ID
         * @param {boolean} enrichApiGateways - determines whether to enrich functions with their related API gateways
         * @returns {Promise}
         */
        function getFunctions(id, enrichApiGateways) {
            return NuclioFunctionsDataService.getFunctions(id, enrichApiGateways);
        }
    }
}());

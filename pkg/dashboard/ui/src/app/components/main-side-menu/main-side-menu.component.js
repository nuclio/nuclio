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
        .component('nclMainSideMenu', {
            templateUrl: 'main-side-menu/main-side-menu.tpl.html',
            controller: NclMainSideMenuController
        });

    function NclMainSideMenuController($state, lodash, ConfigService, NuclioVersionService,
                                       NuclioNamespacesDataService) {
        var ctrl = this;

        ctrl.$onInit = onInit;

        ctrl.isMenuShown = true;
        ctrl.isDemoMode = ConfigService.isDemoMode;
        ctrl.namespaceData = NuclioNamespacesDataService.namespaceData;

        ctrl.isNamespacesExist = isNamespacesExist;
        ctrl.isActiveState = isActiveState;
        ctrl.onDataChange = onDataChange;

        //
        // Hook methods
        //

        /**
         * Initialization function
         */
        function onInit() {
            var origin = sessionStorage.getItem('origin')

            if (origin) {
                ctrl.isMenuShown = false

                angular.element('.ncl-main-wrapper').css('padding-left', '0');
                angular.element('.ncl-main-header').css('left', '0');
            } else {
                NuclioVersionService.getVersion()
                    .then(function (versionInfo) {
                        ctrl.nuclioVersion = lodash.get(versionInfo, 'dashboard.label', 'unknown version');
                    });
            }
        }

        //
        // Public method
        //

        /**
         * Checks if namespaces exist
         * @returns {boolean}
         */
        function isNamespacesExist() {
            return !lodash.isEmpty(ctrl.namespaceData.namespaces);
        }

        /**
         * Checks if current state is active
         * @param {Array.<string>} states
         * @returns {boolean}
         */
        function isActiveState(states) {
            var activeStates = lodash.chain(states).map($state.includes).without(false).value();

            return !lodash.isEmpty(activeStates);
        }

        /**
         * Callback on select item in namespace dropdown
         * @param {Object} item - selected item
         */
        function onDataChange(item) {
            ctrl.namespaceData.selectedNamespace = item;
            localStorage.setItem('namespace', item.name);

            $state.go('app.projects', {namespace: item.name});
        }
    }
}());

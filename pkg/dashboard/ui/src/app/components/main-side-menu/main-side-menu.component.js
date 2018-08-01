(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('nclMainSideMenu', {
            templateUrl: 'main-side-menu/main-side-menu.tpl.html',
            controller: NclMainSideMenuController
        });

    function NclMainSideMenuController($state, lodash, ConfigService, DialogsService, NuclioVersionService, NuclioProjectsDataService) {
        var ctrl = this;

        ctrl.selectedNamespace = null;
        ctrl.namespaces = [];

        ctrl.$onInit = onInit;

        ctrl.isDemoMode = ConfigService.isDemoMode;

        ctrl.isActiveState = isActiveState;
        ctrl.resetNamespaceDropdown = resetNamespaceDropdown;
        ctrl.onDataChange = onDataChange;

        //
        // Hook methods
        //

        /**
         * Initialization function
         */
        function onInit() {
            NuclioVersionService.getVersion()
                .then(function (response) {
                    ctrl.nuclioVersion = lodash.get(response.data, 'dashboard.label', 'unstable');
                });

            NuclioProjectsDataService.getNamespaces()
                .then(function (response) {
                    ctrl.namespaces = lodash.map(response.namespaces.names, function (name) {
                        return {
                            type: 'namespace',
                            id: name,
                            name: name
                        }
                    });
                })
                .catch(function () {
                    DialogsService.alert('Oops: Unknown error occurred while retrieving namespaces');
                });
        }

        //
        // Public method
        //

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
         * Resets namespace dropdown to default state
         */
        function resetNamespaceDropdown() {
            ctrl.selectedNamespace = null;
        }

        /**
         * Callback on select item in namespace dropdown
         * @param {Object} item - selected item
         */
        function onDataChange(item) {
            ctrl.selectedNamespace = item;

            $state.go('app.projects', {namespace: item.name});
        }
    }
}());

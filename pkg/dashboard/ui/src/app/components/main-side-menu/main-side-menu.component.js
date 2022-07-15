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

        ctrl.isDemoMode = ConfigService.isDemoMode;
        ctrl.namespaceData = NuclioNamespacesDataService.namespaceData;

        ctrl.isNamespacesExist = isNamespacesExist;
        ctrl.isActiveState = isActiveState;
        ctrl.onDataChange = onDataChange;

        // Default to nuclio namespace is provided in list of namespaces
        if (!lodash.isEmpty(ctrl.namespaceData)) {
            var nuclioNamespace = lodash.find(ctrl.namespaceData.namespaces, {'name': 'nuclio'});
            if (!lodash.isNil(nuclioNamespace)) {
                ctrl.onDataChange(nuclioNamespace);
            }
        }

        //
        // Hook methods
        //

        /**
         * Initialization function
         */
        function onInit() {
            NuclioVersionService.getVersion()
                .then(function (versionInfo) {
                    ctrl.nuclioVersion = lodash.get(versionInfo, 'dashboard.label', 'unknown version');
                });
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

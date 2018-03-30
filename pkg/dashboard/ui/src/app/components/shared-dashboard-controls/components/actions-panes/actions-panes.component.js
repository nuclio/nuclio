(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('igzActionsPanes', {
            bindings: {
                infoPaneDisable: '<?',
                isInfoPaneOpened: '<?',
                filtersToggleMethod: '&?',
                filtersCounter: '<?',
                showFilterIcon: '@?',
                infoPaneToggleMethod: '&?',
                closeInfoPane: '&?'
            },
            templateUrl: 'shared-dashboard-controls/components/actions-panes/actions-panes.tpl.html',
            controller: IgzActionsPanesController
        });

    function IgzActionsPanesController(lodash, ConfigService) {
        var ctrl = this;

        ctrl.callToggleMethod = null;

        ctrl.$onInit = onInit;

        ctrl.isShowFilterActionIcon = isShowFilterActionIcon;

        //
        // Hook methods
        //

        /**
         * Initialization method
         */
        function onInit() {
            ctrl.callToggleMethod = angular.isFunction(ctrl.closeInfoPane) ? ctrl.closeInfoPane : ctrl.infoPaneToggleMethod;
        }

        //
        // Public method
        //

        /**
         * Checks if filter toggles method exists and if filter pane should toggle only in demo mode
         * @returns {boolean}
         */
        function isShowFilterActionIcon() {
            return angular.isFunction(ctrl.filtersToggleMethod) && (lodash.isEqual(ctrl.showFilterIcon, 'true') || ConfigService.isDemoMode());
        }
    }
}());

(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('igzActionPanel', {
            bindings: {
                actions: '<',
                onItemsCheckedCount: '&?'
            },
            templateUrl: 'shared-dashboard-controls/components/action-panel/action-panel.tpl.html',
            controller: IgzActionPanel,
            transclude: true
        });

    function IgzActionPanel($scope, $rootScope, lodash, LoginService) {
        var ctrl = this;

        var checkedItemsCount = 0;
        var mainActionsCount = 5;

        ctrl.mainActions = [];
        ctrl.remainActions = [];

        ctrl.$onInit = onInit;
        ctrl.isActionPanelShown = isActionPanelShown;

        //
        // Hook methods
        //

        /**
         * Initialization method
         */
        function onInit() {
            $scope.$on('action-checkbox-all_checked-items-count-change', onUpdateCheckedItemsCount);
            $scope.$on('action-checkbox-all_check-all', onUpdateCheckedItemsCount);
            ctrl.actions = lodash.filter(ctrl.actions, function (action) {
                return !lodash.has(action, 'capability') || LoginService.hasCapabilities(action.capability);
            });
            angular.forEach(ctrl.actions, function (action) {
                if (!angular.isFunction(action.handler)) {
                    action.handler = defaultAction;

                    if (action.id === 'delete' && angular.isUndefined(action.confirm)) {
                        action.confirm = {
                            message: 'Are you sure you want to delete selected items?',
                            yesLabel: 'Yes, Delete',
                            noLabel: 'Cancel',
                            type: 'critical_alert'
                        };
                    }
                }
            });
            ctrl.mainActions = lodash.slice(ctrl.actions, 0, mainActionsCount);
            ctrl.remainingActions = lodash.slice(ctrl.actions, mainActionsCount, ctrl.actions.length);
        }

        //
        // Private methods
        //

        /**
         * Default action handler
         * @param {Object} action
         * @param {string} action.id - an action ID (e.g. delete, clone etc.)
         */
        function defaultAction(action) {
            $rootScope.$broadcast('action-panel_fire-action', {
                action: action.id
            });
        }

        /**
         * Checks whether the action panel can be shown
         * @returns {boolean}
         */
        function isActionPanelShown() {
            return checkedItemsCount > 0;
        }

        /**
         * Called when 'Check all' checkbox is clicked or checked some item.
         * @param {Event} event - $broadcast-ed event
         * @param {Object} data - $broadcast-ed data
         * @param {Object} data.checkedCount - count of checked items
         */
        function onUpdateCheckedItemsCount(event, data) {
            checkedItemsCount = data.checkedCount;

            if (angular.isFunction(ctrl.onItemsCheckedCount)) {
                ctrl.onItemsCheckedCount({checkedCount: checkedItemsCount});
            }

            var visibleActions = lodash.filter(ctrl.actions, ['visible', true]);

            ctrl.mainActions = lodash.slice(visibleActions, 0, mainActionsCount);
            ctrl.remainingActions = lodash.slice(visibleActions, mainActionsCount, visibleActions.length);
        }
    }
}());

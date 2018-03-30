(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('igzActionCheckbox', {
            bindings: {
                item: '<',
                onClickCallback: '&?'
            },
            templateUrl: 'shared-dashboard-controls/components/action-checkbox/action-checkbox.tpl.html',
            controller: IgzActionCheckbox
        });

    function IgzActionCheckbox($scope, $rootScope) {
        var ctrl = this;

        ctrl.onCheck = onCheck;
        ctrl.$onInit = $onInit;

        //
        // Public methods
        //

        /**
         * Handles mouse click on checkbox
         * @param {Object} $event - event object
         */
        function onCheck($event) {
            ctrl.item.ui.checked = !ctrl.item.ui.checked;

            if (angular.isFunction(ctrl.onClickCallback)) {
                $event.stopPropagation();
                ctrl.onClickCallback();
            }

            $rootScope.$broadcast('action-checkbox_item-checked', {checked: ctrl.item.ui.checked});
        }

        //
        // Private methods
        //

        /**
         * Constructor method
         */
        function $onInit() {
            $scope.$on('action-checkbox-all_check-all', toggleCheckedAll);
        }

        /**
         * Triggers on Check all button clicked
         * @param {Object} event
         * @param {Object} data
         */
        function toggleCheckedAll(event, data) {
            if (ctrl.item.ui.checked !== data.checked) {
                ctrl.item.ui.checked = !ctrl.item.ui.checked;
            }

            if (angular.isFunction(ctrl.onClickCallback)) {
                ctrl.onClickCallback();
            }
        }
    }
}());

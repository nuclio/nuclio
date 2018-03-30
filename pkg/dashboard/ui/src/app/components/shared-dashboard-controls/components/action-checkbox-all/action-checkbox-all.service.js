(function () {
    'use strict';

    angular.module('iguazio.app')
        .factory('ActionCheckboxAllService', ActionCheckboxAllService);

    function ActionCheckboxAllService($rootScope) {
        return {
            changeCheckedItemsCount: changeCheckedItemsCount,
            setCheckedItemsCount: setCheckedItemsCount
        };

        //
        // Public methods
        //

        /**
         * Sends broadcast with count of changed checked items
         * @param {number} changedCheckedItemsCount - number of changed checked items
         */
        function changeCheckedItemsCount(changedCheckedItemsCount) {
            $rootScope.$broadcast('action-checkbox-all_change-checked-items-count', {
                changedCheckedItemsCount: changedCheckedItemsCount
            });
        }

        /**
         * Sends broadcast with count of checked items
         * @param {number} checkedItemsCount
         */
        function setCheckedItemsCount(checkedItemsCount) {
            $rootScope.$broadcast('action-checkbox-all_set-checked-items-count', checkedItemsCount);
        }
    }
}());

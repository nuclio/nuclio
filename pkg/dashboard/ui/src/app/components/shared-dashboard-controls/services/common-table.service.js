(function () {
    'use strict';

    angular.module('iguazio.app')
        .factory('CommonTableService', CommonTableService);

    function CommonTableService() {
        return {
            isColumnSorted: isColumnSorted
        };

        //
        // Public methods
        //

        /**
         * Checks whether the passed column name equals the last sorted column name
         * @param {string} columnName
         * @param {string} lastSortedColumnName
         * @param {boolean} isReversed
         * @return {{sorted: boolean, reversed: boolean}} - an object with css class names suitable for `ng-class`
         */
        function isColumnSorted(columnName, lastSortedColumnName, isReversed) {
            var classes = {
                'sorted': false,
                'reversed': false
            };
            if (columnName === lastSortedColumnName) {
                classes.sorted = true;
                classes.reversed = isReversed;
            }
            return classes;
        }
    }
}());

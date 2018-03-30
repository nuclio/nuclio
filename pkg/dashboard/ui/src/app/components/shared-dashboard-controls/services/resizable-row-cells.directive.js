(function () {
    'use strict';

    angular.module('iguazio.app')
        .directive('igzResizableRowCells', igzResizableRowCells);

    function igzResizableRowCells($rootScope, $timeout, lodash) {
        return {
            restrict: 'A',
            link: link
        };

        function link(scope, element) {
            activate();

            //
            // Private methods
            //

            /**
             * Constructor
             */
            function activate() {
                scope.$on('resize-cells', resizeCells);
                scope.$on('autofit-col', autoFitColumn);

                $timeout(function () {
                    $rootScope.$broadcast('reload-columns');
                });
            }

            /**
             * Checks width of cells column auto-fit
             * @param {Object} event - broadcast event
             * @param {Object} data - information about column name
             */
            function autoFitColumn(event, data) {
                var currentCell = element.find('.' + data.colClass)[0];

                if (!lodash.isNil(currentCell)) {
                    var currentWidth = currentCell.offsetWidth;

                    // temporary set auto width to get data for auto-fit
                    currentCell.style.width = 'auto';
                    var newWidth = currentCell.offsetWidth;
                    currentCell.style.width = currentWidth + 'px';

                    if (newWidth > currentWidth) {
                        var colDifference = newWidth - currentWidth + 2;

                        if (angular.isFunction(data.callbackFunction)) {
                            data.callbackFunction(colDifference);
                        }
                    }
                }
            }

            /**
             * Resize cells according to igz-resize-table-column directive move
             * @param {Object} event - broadcast event
             * @param {Object} data - information about column name and size
             */
            function resizeCells(event, data) {

                // search for cell which should be resized
                var currentCell = element.find('.' + data.colClass)[0];
                if (!lodash.isNil(currentCell)) {
                    var nextCell = currentCell.nextElementSibling;

                    // set new value for cells width
                    currentCell.style.width = data.columnWidth;
                    nextCell.style.width = data.nextColumnWidth;
                }
            }
        }
    }
}());


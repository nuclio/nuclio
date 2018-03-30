(function () {
    'use strict';

    angular.module('iguazio.app')
        .directive('igzResizableTableColumn', igzResizableTableColumn);

    function igzResizableTableColumn($document, $rootScope, $timeout, $window, lodash) {
        return {
            restrict: 'A',
            replace: true,
            scope: {
                colClass: '@'
            },
            template: '<div class="resize-block" data-ng-mousedown="resizeTable.onMouseDown($event)" data-ng-click="resizeTable.onClick($event)" data-ng-dblclick="resizeTable.onDoubleClick($event)"></div>',
            controller: IgzResizeTableController,
            controllerAs: 'resizeTable',
            bindToController: true
        };

        function IgzResizeTableController($element, $scope) {
            var vm = this;

            vm.minWidth = 100;
            vm.startPosition = 0;

            vm.onMouseDown = onMouseDown;
            vm.onClick = onClick;
            vm.onDoubleClick = onDoubleClick;

            activate();

            //
            // Public methods
            //

            /**
             * Prevents click propagation
             * @param {Object} event
             */
            function onClick(event) {
                event.stopPropagation();
            }

            /**
             * Prevents click propagation
             * @param {Object} event
             */
            function onDoubleClick(event) {

                // set min width for selected column
                if (vm.columnHeadMinWidth < vm.columnHeadWidth) {
                    var colDifference = vm.columnHeadMinWidth - vm.columnHeadWidth;
                    resizeColumn(colDifference);
                }

                // set width of the column to fit the content
                $rootScope.$broadcast('autofit-col', {colClass: vm.colClass, callbackFunction: resizeColumn});
            }

            /**
             * On mouse down handler
             * @param {Object} event
             */
            function onMouseDown(event) {

                // prevent default dragging of selected content
                event.preventDefault();
                event.stopPropagation();

                // saves start position of resize
                vm.startPosition = event.clientX;

                // adds event listeners
                $document.on('mousemove', onMouseMove);
                $document.on('mouseup', onMouseUp);

                return false;
            }

            //
            // Private methods
            //

            /**
             * Constructor
             */
            function activate() {

                // set header widths of the resizing columns
                $timeout(initColumnsWidths);

                angular.element($window).on('resize', reloadColumns);
                $scope.$on('reload-columns', reloadColumns);
                $scope.$on('$destroy', destructor);
            }

            /**
             * Destructor method
             */
            function destructor() {
                angular.element($window).off('resize', reloadColumns);
            }

            /**
             * On mouse move handler
             * @param {Object} event
             */
            function onMouseMove(event) {
                var colDifference = event.clientX - vm.startPosition;
                vm.startPosition = event.clientX;
                resetColumnsWidths();
                resizeColumn(colDifference);
            }

            /**
             * On mouse up handlers
             * @param {Object} event
             */
            function onMouseUp(event) {

                // detaches even listeners
                $document.off('mousemove', onMouseMove);
                $document.off('mouseup', onMouseUp);

                // prevent default dragging of selected content
                event.preventDefault();
                event.stopPropagation();

                $rootScope.$broadcast('resize-tags-cells');
            }

            /**
             * Reloads column cells in the table according to column width
             */
            function reloadColumns() {
                if (!lodash.isNil(vm.nextBlock)) {
                    $timeout(function () {
                        resetColumnsWidths();

                        $rootScope.$broadcast('resize-cells', {colClass: vm.colClass, columnWidth: vm.columnHeadWidth + 'px', nextColumnWidth: vm.nextBlockWidth + 'px'});
                    });
                }
            }

            /**
             * Initialises columns and their min width
             */
            function initColumnsWidths() {

                // get block which will be resized
                vm.columnHead = $element[0].parentElement;
                vm.columnHeadMinWidth = vm.minWidth;
                if (vm.columnHead.offsetWidth > 0) {
                    vm.columnHeadMinWidth = lodash.min([vm.columnHead.offsetWidth, vm.minWidth]);
                }

                // get parent container of the header
                vm.parentBlock = vm.columnHead.parentElement;

                // get block which is next to resizing block
                vm.nextBlock = vm.columnHead.nextElementSibling;
                vm.nextBlockMinWidth = vm.minWidth;
                if (!lodash.isNil(vm.nextBlock) && vm.nextBlock.offsetWidth > 0) {
                    vm.nextBlockMinWidth = lodash.min([vm.nextBlock.offsetWidth, vm.minWidth]);
                }
                resetColumnsWidths();
            }

            /**
             * Resets columns widths
             */
            function resetColumnsWidths() {
                vm.columnHeadWidth = vm.columnHead.offsetWidth;
                vm.parentBlockWidth = vm.parentBlock.offsetWidth;
                if (!lodash.isNil(vm.nextBlock)) {
                    vm.nextBlockWidth = vm.nextBlock.offsetWidth;
                }
            }

            /**
             * Resize cells in the table rows according to column width
             * @param {Object} data - information about column name and difference
             */
            function resizeColumn(colDifference) {
                if (!lodash.isNil(vm.nextBlock)) {

                    // calculate new width for the block which need to be resized
                    var maxColumnHeadDifference = vm.columnHeadWidth - vm.columnHeadMinWidth;

                    // calculate new width for the  block which is next to resizing block
                    var maxNextBlockDifference = vm.nextBlockWidth - vm.nextBlockMinWidth;

                    // calculate maximum resizing value of columns
                    var newDifference = 0;
                    if (colDifference > 0 && maxNextBlockDifference > 0) {
                        newDifference = lodash.min([colDifference, maxNextBlockDifference]);
                    } else if (colDifference < 0 && maxColumnHeadDifference > 0) {
                        newDifference = lodash.max([colDifference, -maxColumnHeadDifference]);
                    }

                    if (newDifference !== 0) {
                        vm.columnHeadWidth = vm.columnHeadWidth + newDifference;
                        vm.nextBlockWidth = vm.nextBlockWidth - newDifference;

                        setElementWidth(vm.columnHead, vm.columnHeadWidth);
                        setElementWidth(vm.nextBlock, vm.nextBlockWidth);

                        $rootScope.$broadcast('resize-cells', {
                            colClass: vm.colClass,
                            columnWidth: vm.columnHeadWidth + 'px',
                            nextColumnWidth: vm.nextBlockWidth + 'px'
                        });
                        $rootScope.$broadcast('resize-size-cells');
                    }
                }
            }

            /**
             * Sets header element width in percentage
             * @param {Object} element - element object
             * @param {number} widthInPixels - new width value
             */
            function setElementWidth(element, widthInPixels) {
                element.style.width = (widthInPixels / vm.parentBlockWidth * 100) + '%';
            }
        }
    }

}());

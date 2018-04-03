(function () {
    'use strict';

    angular.module('iguazio.app')
        .directive('igzNgScrollbarsMethods', igzNgScrollbarsMethods);

    function igzNgScrollbarsMethods() {
        return {
            restrict: 'A',
            link: link
        };

        function link(scope, element, attr) {
            activate();

            /**
             * Constructor method
             */
            function activate() {
                scope.$on('ng-scrollbars-methods_scroll-to', scrollToTarget);
            }

            /**
             * Scrolls to specified element (selected by id) inside ng-scrollbars container
             * Used in broadcast
             * @param {Object} event - broadcast event
             * @param {Object} data - broadcast data
             */
            function scrollToTarget(event, data) {

                // Check if current directive was demanded
                if (attr.igzNgScrollbarsMethods === data.scrollContainerName) {
                    element.mCustomScrollbar('scrollTo', '#' + data.target);
                }
            }
        }
    }
}());

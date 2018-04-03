(function () {
    'use strict';

    /**
     * Extend white background to the bottom of the view port
     */
    angular.module('iguazio.app')
        .directive('igzExtendBackground', igzExtendBackground);

    function igzExtendBackground($timeout) {
        return {
            restrict: 'A',
            link: link
        };

        function link(scope, element, attrs) {
            var timeout = 0;
            var containerPath = 'body';

            activate();

            //
            // Private methods
            //

            /**
             * Constructor method
             */
            function activate() {
                timeout = Number(attrs.igzExtendBackground) || 0;
                containerPath = attrs.containerPath || 'body';

                $timeout(elementMinHeight, timeout);
                scope.$on('igzWatchWindowResize::resize', elementMinHeight);
            }

            /**
             * Calculate and change element height
             */
            function elementMinHeight() {
                var container = angular.element(containerPath);
                var containerBox = container[0].getBoundingClientRect();
                var paddingBottom = parseInt(container.css('padding-bottom'), 10);
                var box = element[0].getBoundingClientRect();

                if (containerBox.height === 0) {
                    element.css('height', '100%');
                    element.css('padding-bottom', '45px');
                } else {
                    element.css('padding-bottom', '0');
                    element.css('height', (containerBox.bottom + paddingBottom - box.top) + 'px');
                }
            }
        }
    }
}());

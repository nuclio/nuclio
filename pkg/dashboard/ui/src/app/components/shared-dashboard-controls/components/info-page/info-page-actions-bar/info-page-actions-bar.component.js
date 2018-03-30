(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('igzInfoPageActionsBar', {
            bindings: {
                watchId: '@?'
            },
            templateUrl: 'shared-dashboard-controls/components/info-page/info-page-actions-bar/info-page-actions-bar.tpl.html',
            transclude: true,
            controller: IgzInfoPageActionsBarController
        });

    function IgzInfoPageActionsBarController($scope) {
        var ctrl = this;

        ctrl.isUpperPaneShowed = false;
        ctrl.isFiltersShowed = false;
        ctrl.isInfoPaneShowed = false;

        ctrl.$onInit = onInit;

        //
        // Hook methods
        //

        /**
         * Init method
         */
        function onInit() {
            var watchId = angular.isDefined(ctrl.watchId) ? '-' + ctrl.watchId : '';

            $scope.$on('info-page-upper-pane_toggle-start' + watchId, onUpperPaneToggleStart);
            $scope.$on('info-page-filters_toggle-start' + watchId, onFiltersPaneToggleStart);
            $scope.$on('info-page-pane_toggle-start' + watchId, onInfoPaneToggleStart);
        }

        //
        // Private methods
        //

        /**
         * Upper pane toggle start $broadcast listener
         * @param {Object} e - broadcast event
         * @param {boolean} isShown - represents upper pane state
         */
        function onUpperPaneToggleStart(e, isShown) {
            ctrl.isUpperPaneShowed = isShown;
        }

        /**
         * Filters pane toggle start $broadcast listener
         * @param {Object} e - broadcast event
         * @param {boolean} isShown - represents filters pane state
         */
        function onFiltersPaneToggleStart(e, isShown) {
            ctrl.isFiltersShowed = isShown;
        }

        /**
         * Info pane toggle start $broadcast listener
         * @param {Object} e - broadcast event
         * @param {boolean} isShown - represents info pane state
         */
        function onInfoPaneToggleStart(e, isShown) {
            ctrl.isInfoPaneShowed = isShown;
        }

    }
}());

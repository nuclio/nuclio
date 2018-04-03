(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('igzSplashScreen', {
            bindings: {
                isSplashShowed: '<'
            },
            templateUrl: 'shared-dashboard-controls/components/splash-screen/splash-screen.tpl.html',
            controller: IgzSplashScreenController
        });

    function IgzSplashScreenController($scope, $state) {
        var ctrl = this;

        // public properties
        ctrl.isLoading = true;
        ctrl.isAlertShowing = false;
        ctrl.textToDisplay = 'Loading…';

        ctrl.$onInit = onInit;

        // public methods
        ctrl.refreshPage = refreshPage;

        //
        // Hook methods
        //

        /**
         * Initialization method
         */
        function onInit() {
            $scope.$on('splash-screen_show-error', showError);
            $scope.$on('browse-action_change-loading-text', changeLoadingText);
        }

        //
        // Public methods
        //

        /**
         * Sends broadcast to refresh browse page
         */
        function refreshPage() {
            ctrl.isLoading = true;
            ctrl.isAlertShowing = false;

            $state.reload();
        }

        //
        // Private methods
        //

        /**
         * Changes displayed text on loading spinner
         * @param {Object} event - broadcast event
         * @param {Object} data - broadcast data with text to be displayed
         */
        function changeLoadingText(event, data) {
            ctrl.textToDisplay = data.textToDisplay;
        }


        /**
         * Shows error text
         * @param {Object} event - native broadcast event
         * @param {string} data - broadcast data
         */
        function showError(event, data) {
            if (angular.isDefined(data.textToDisplay)) {
                ctrl.textToDisplay = data.textToDisplay;
            }

            if (angular.isDefined(data.alertText)) {
                ctrl.alertText = data.alertText;
            }

            ctrl.isLoading = false;
            ctrl.isAlertShowing = true;
        }
    }
}());

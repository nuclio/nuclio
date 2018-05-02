(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('nclHeader', {
            templateUrl: 'header/header.tpl.html',
            controller: NclHeaderController
        });

    function NclHeaderController($timeout, $element, $rootScope, $scope, $state, $window, ngDialog, lodash, ConfigService,
                                 NavigationTabsService) {
        var ctrl = this;

        var topGeneralContentPosition;
        var headerOffsetTopPosition;

        ctrl.isSplashShowed = {
            value: false
        };
        ctrl.mainHeaderTitle = {};
        ctrl.navigationTabsConfig = [];
        ctrl.newRunningTasks = false;
        ctrl.newCriticalAlerts = false;
        ctrl.isHeaderExpanded = true;

        ctrl.$onInit = onInit;
        ctrl.$postLink = postLink;

        ctrl.isDemoMode = ConfigService.isDemoMode;
        ctrl.isStagingMode = ConfigService.isStagingMode;
        ctrl.onOpenAlertsDropdown = onOpenAlertsDropdown;
        ctrl.openSettingsDialog = openSettingsDialog;
        ctrl.onToggleHeader = onToggleHeader;
        ctrl.isNuclioState = isNuclioState;

        //
        // Hook methods
        //

        /**
         * Initialization function
         */
        function onInit() {
            $scope.$on('$stateChangeSuccess', onStateChangeSuccess);
            $scope.$on('trigger-logout-splash', triggerSplash);
            $scope.$on('change-count-of-critical-alerts', changeCountOfCriticalAlerts);
            $scope.$on('change-count-of-running-tasks', changeCountOfRunningTasks);
        }

        /**
         * Post linking method
         */
        function postLink() {
            ctrl.navigationTabsConfig = NavigationTabsService.getNavigationTabsConfig($state.current.name);

            $timeout(function () {
                setMainWrapperPosition();

                topGeneralContentPosition = angular.element('.igz-general-content').css('top');
                headerOffsetTopPosition = $element.find('.ncl-main-header').get(0).clientHeight;
            });
        }

        //
        // Public methods
        //

        /**
         * Checks if it is the Nuclio state
         * @returns {boolean}
         */
        function isNuclioState() {
            return lodash.includes($state.current.name, 'app.project');
        }

        /**
         * Resizes height of alerts dropdown depends on window size
         */
        function onOpenAlertsDropdown() {
            var alertsMenuHeight = Number($element.find('.top-alerts-menu').css('height').replace('px', ''));
            var alertsScrollableContainerHeight = Number($element.find('.alerts-scrollable-container').css('height').replace('px', ''));
            var bottomSpaceHeight = 20;
            var difference = alertsMenuHeight - alertsScrollableContainerHeight;
            var topAlertsListHeight = Number($element.find('.top-alerts-list').css('height').replace('px', ''));

            $element.find('.alerts-scrollable-container').css('height', $window.innerHeight - $element[0].offsetTop - difference - bottomSpaceHeight);

            alertsScrollableContainerHeight = Number($element.find('.alerts-scrollable-container').css('height').replace('px', ''));

            if (topAlertsListHeight < alertsScrollableContainerHeight) {
                $element.find('.alerts-scrollable-container').css('height', 'auto');
            }
        }

        /**
         * Opens settings dialog
         */
        function openSettingsDialog() {
            ngDialog.open({
                template: '<igz-settings-dialog data-close-dialog="closeThisDialog()"></igz-settings-dialog>',
                plain: true,
                scope: $scope,
                className: 'ngdialog-theme-iguazio settings-dialog-wrapper'
            });
        }

        /**
         * Toggles header
         */
        function onToggleHeader() {
            ctrl.isHeaderExpanded = !ctrl.isHeaderExpanded;
            $rootScope.$broadcast('header_toggle-start', ctrl.isHeaderExpanded);

            var generalContent = angular.element('.igz-general-content');
            var generalContentZIndex = !ctrl.isHeaderExpanded ? '998' : '996';

            if (!ctrl.isHeaderExpanded) {
                generalContent.css('z-index', generalContentZIndex);
            }

            generalContent.animate({
                'top': !ctrl.isHeaderExpanded ? -headerOffsetTopPosition : topGeneralContentPosition
            }, 300, function () {
                var dispayingExpansionButton = ctrl.isHeaderExpanded ? '' : 'flex';

                if (ctrl.isHeaderExpanded) {
                    generalContent.css('z-index', generalContentZIndex);
                }

                $element.find('.header-expansion-button').css('display', dispayingExpansionButton);

                $rootScope.$broadcast('igzWatchWindowResize::resize');
            });
        }

        //
        // Private methods
        //

        /**
         * Changes count of critical alerts
         * @param {Event} event
         * @param {number} countOfAlerts
         */
        function changeCountOfCriticalAlerts(event, countOfAlerts) {
            ctrl.newCriticalAlerts = countOfAlerts > 0;
        }

        /**
         * Changes count of running tasks
         * @param {Event} event
         * @param {number} countOfTasks
         */
        function changeCountOfRunningTasks(event, countOfTasks) {
            ctrl.newRunningTasks = countOfTasks > 0;
        }

        /**
         * Dynamically pre-set Main Header Title on UI router state change, sets position of main wrapper and navigation
         * tabs config
         * Needed for better UX - header title changes correctly even before controller data resolved and broadcast
         * have been sent
         * @param {Object} event
         * @param {Object} toState
         */
        function onStateChangeSuccess(event, toState) {
            ctrl.navigationTabsConfig = NavigationTabsService.getNavigationTabsConfig(toState.name);

            $timeout(function () {
                var generalContentZIndex = '998';

                setMainWrapperPosition();

                headerOffsetTopPosition = $element.find('.ncl-main-header').get(0).clientHeight;

                if (!ctrl.isHeaderExpanded) {
                    angular.element('.igz-general-content').css('z-index', generalContentZIndex);
                    angular.element('.igz-general-content').css('top', -headerOffsetTopPosition);
                }
            });
        }

        /**
         * Sets padding-top of igz-main-wrapper depending on height of header
         */
        function setMainWrapperPosition() {
            var mainWrapperPaddingTop = $element.find('.ncl-main-header').get(0).clientHeight;

            angular.element('.ncl-main-wrapper').css('padding-top', mainWrapperPaddingTop + 'px');
        }

        /**
         * Show/Hide splash screen
         */
        function triggerSplash() {
            ctrl.isSplashShowed.value = !ctrl.isSplashShowed.value;
        }
    }
}());

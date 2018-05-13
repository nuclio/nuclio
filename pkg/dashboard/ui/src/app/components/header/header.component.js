(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('nclHeader', {
            templateUrl: 'header/header.tpl.html',
            controller: NclHeaderController
        });

    function NclHeaderController($timeout, $element, $rootScope, $scope, $state, $window, ngDialog, lodash, ConfigService,
                                 DialogsService, NuclioVersionService) {
        var ctrl = this;

        var topGeneralContentPosition;
        var headerOffsetTopPosition;

        ctrl.isSplashShowed = {
            value: false
        };
        ctrl.mainHeaderTitle = {};
        ctrl.navigationTabsConfig = [];
        ctrl.isHeaderExpanded = true;

        ctrl.$onInit = onInit;
        ctrl.$postLink = postLink;

        ctrl.isDemoMode = ConfigService.isDemoMode;
        ctrl.isStagingMode = ConfigService.isStagingMode;
        ctrl.showVersion = showVersion;
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
        }

        /**
         * Post linking method
         */
        function postLink() {
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
         * Shows a popup with current version of Nuclio
         */
        function showVersion() {
            NuclioVersionService.getVersion()
                .then(function (response) {

                    // open dialog with detail information about Nuclio's version
                    ngDialog.open({
                        template: '<ncl-version-info-dialog data-close-dialog="closeThisDialog()" ' +
                        'data-version="ngDialogData.version"></ncl-version-info-dialog>',
                        plain: true,
                        scope: $scope,
                        data: {
                            version: response.data
                        },
                        className: 'ngdialog-theme-nuclio'
                    });
                })
                .catch(function () {
                    DialogsService.alert('Oops: Unknown error occurred while getting Nuclio\'s version');
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
         * Dynamically pre-set Main Header Title on UI router state change, sets position of main wrapper and navigation
         * tabs config
         * Needed for better UX - header title changes correctly even before controller data resolved and broadcast
         * have been sent
         * @param {Object} event
         * @param {Object} toState
         */
        function onStateChangeSuccess(event, toState) {

            $timeout(function () {
                setMainWrapperPosition();

                headerOffsetTopPosition = $element.find('.ncl-main-header').get(0).clientHeight;
            });
        }

        /**
         * Sets padding-top of igz-main-wrapper depending on height of header
         */
        function setMainWrapperPosition() {
            var mainWrapperPaddingTop = $element.find('.ncl-main-header').get(0).clientHeight;

            angular.element('.ncl-main-wrapper').css('padding-top', mainWrapperPaddingTop + 'px');
        }
    }
}());

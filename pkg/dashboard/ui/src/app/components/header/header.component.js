(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('nclHeader', {
            templateUrl: 'header/header.tpl.html',
            controller: NclHeaderController
        });

    function NclHeaderController($timeout, $element, $rootScope, $scope, $state, $transitions, $i18next, i18next,
                                 lodash, ConfigService) {
        var ctrl = this;

        var deregisterExitFunction = null;
        var deregisterErrorFunction = null;
        var topGeneralContentPosition;
        var headerOffsetTopPosition;

        ctrl.isSplashShowed = {
            value: false
        };
        ctrl.languages = [
            {
                name: 'English',
                id: 'en'
            }
        ];
        ctrl.mainHeaderTitle = {};
        ctrl.navigationTabsConfig = [];
        ctrl.isHeaderExpanded = true;

        ctrl.$onInit = onInit;
        ctrl.$postLink = postLink;

        ctrl.isDemoMode = ConfigService.isDemoMode;
        ctrl.isStagingMode = ConfigService.isStagingMode;
        ctrl.isNuclioState = isNuclioState;
        ctrl.onLanguageChange = onLanguageChange;
        ctrl.onToggleHeader = onToggleHeader;

        //
        // Hook methods
        //

        /**
         * Initialization function
         */
        function onInit() {
            $scope.$on('$stateChangeSuccess', onStateChangeSuccess);
            setSelectedLanguage();
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
         * Callback on changing language
         * @param {Object} item - selected language object
         * @param {boolean} isItemChanged - was value changed or not
         */
        function onLanguageChange(item, isItemChanged) {
            if (isItemChanged) {
                ctrl.selectedLanguage = item;
                i18next.loadLanguages(ctrl.selectedLanguage.id, function () {
                    $state.reload();

                    deregisterExitFunction = $transitions.onExit({}, function () {
                        $i18next.changeLanguage(ctrl.selectedLanguage.id);
                        deregisterExitFunction();
                        deregisterErrorFunction();
                    });

                    deregisterErrorFunction = $transitions.onError({}, function () {
                        setSelectedLanguage();
                        deregisterErrorFunction();
                        deregisterExitFunction();
                    });
                });
            }
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

        /**
         * Sets current language as selected
         */
        function setSelectedLanguage() {
            ctrl.selectedLanguage = lodash.find(ctrl.languages, ['id', i18next.language.replace(/(\w{2}).+/, '$1')]);
        }
    }
}());

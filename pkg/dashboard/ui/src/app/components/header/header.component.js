/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('nclHeader', {
            templateUrl: 'header/header.tpl.html',
            controller: NclHeaderController
        });

    function NclHeaderController($timeout, $element, $rootScope, $scope, $state, $transitions, $i18next, i18next,
                                 lodash, ConfigService, NavigationTabsService) {
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
                name: 'EN',
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
            $transitions.onSuccess({}, onStateChangeSuccess);
            setSelectedLanguage();
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
         * @param {Object} transition
         */
        function onStateChangeSuccess(transition) {
            ctrl.navigationTabsConfig = NavigationTabsService.getNavigationTabsConfig(transition.$to().name);

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
            var initialLng = i18next.language.replace(/(\w{2}).+/, '$1');
            ctrl.selectedLanguage = lodash.find(ctrl.languages, ['id', initialLng]);

            if (!ctrl.selectedLanguage) {
                ctrl.selectedLanguage = lodash.find(ctrl.languages, ['id', 'en']);
                $i18next.changeLanguage(ctrl.selectedLanguage.id);
            }
        }
    }
}());

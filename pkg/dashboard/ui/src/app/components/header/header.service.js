(function () {
    'use strict';

    angular.module('nuclio.app')
        .factory('HeaderService', HeaderService);

    function HeaderService($timeout, $rootScope, $state, lodash) {
        return {
            updateMainHeader: updateMainHeader
        };

        //
        // Public methods
        //

        /**
         * Sends broadcast with needed data object to dynamically update main header title
         * @param {string} title
         * @param {string} subtitle
         * @param {string} state
         */
        function updateMainHeader(title, subtitle, state) {
            var mainHeaderState = lodash.find($state.get(), function (mainState) {
                return mainState.url === lodash.trim($state.$current.url.prefix, '/');
            }).name;

            var mainHeaderTitle = {
                title: title,
                subtitle: subtitle,
                state: state,
                mainHeaderState: mainHeaderState
            };

            $timeout(function () {
                $rootScope.$broadcast('update-main-header-title', mainHeaderTitle)
            });
        }
    }
}());

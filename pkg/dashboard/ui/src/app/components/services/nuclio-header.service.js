(function () {
    'use strict';

    angular.module('iguazio.app')
        .factory('NuclioHeaderService', NuclioHeaderService);

    function NuclioHeaderService($timeout, $rootScope, $state, lodash) {
        return {
            updateMainHeader: updateMainHeader
        };

        //
        // Public methods
        //

        /**
         * Sends broadcast with needed data object to dynamically update main header title
         * @param {string} title
         * @param {string} subtitles
         * @param {string} state
         */
        function updateMainHeader(title, subtitles, state) {
            var mainHeaderState = lodash.find($state.get(), function (mainState) {
                return mainState.url === lodash.trim($state.$current.url.prefix, '/');
            }).name;

            var mainHeaderTitle = {
                title: title,
                project: subtitles.project,
                function: null,
                version: null,
                state: state,
                mainHeaderState: mainHeaderState
            };

            if (!lodash.isNil(subtitles.function)) {
                mainHeaderTitle.function = subtitles.function;
            }

            if (!lodash.isNil(subtitles.version)) {
                mainHeaderTitle.version = subtitles.version;
            }

            $rootScope.$broadcast('update-main-header-title', mainHeaderTitle);
        }
    }
}());

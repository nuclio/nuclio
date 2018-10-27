(function () {
    'use strict';

    angular.module('nuclio.app')
        .factory('NuclioClientService', NuclioClientService);

    function NuclioClientService($http, lodash, ConfigService) {

        var service = {
            buildUrlWithPath: buildUrlWithPath,
            makeRequest: makeRequest,
            isLoading: {value: false}
        };

        return service;

        //
        // Public methods
        //

        /**
         * Makes request to nuclio server
         * Provides mechanism that allows to check if the request in the progress
         * @param {Object} config - describing the request to be made and how it should be processed (for `$http`
         *     service)
         * @param {boolean} [dataOnly=true] - returns the `data` property of the xhr object. Set to `false` in order to
         *     get the entire xhr object as-is.
         * @param {boolean} [showLoaderOnStart=false] - avoid showing splash screen when request is started, if `true`
         * @param {boolean} [hideLoaderOnEnd=false] - avoid hiding splash screen when request is finished, if `true`
         * @returns {*}
         */
        function makeRequest(config, dataOnly, showLoaderOnStart, hideLoaderOnEnd) {
            if (!showLoaderOnStart) {
                service.isLoading.value = true;
            }

            return $http(config)
                .then(function (response) {
                    if (!showLoaderOnStart && !hideLoaderOnEnd) {
                        service.isLoading.value = false;
                    }

                    return lodash.defaultTo(dataOnly, true) ? response.data : response;
                });
        }

        /**
         * Builds absolute url with a file path
         * @param {string} itemId
         * @param {string} [path]
         * @returns {string}
         */
        function buildUrlWithPath(itemId, path) {
            return buildUrl(itemId) + lodash.trimStart(lodash.defaultTo(path, ''), '/ ');
        }

        //
        // Private methods
        //

        /**
         * Builds absolute url
         * @param {string} itemId
         * @returns {string}
         */
        function buildUrl(itemId) {
            return lodash.trimEnd(ConfigService.url.nuclio.baseUrl, ' /') + '/' + itemId;
        }
    }
}());

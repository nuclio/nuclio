(function () {
    'use strict';

    angular.module('nuclio.app')
        .factory('NuclioVersionService', NuclioVersionService);

    function NuclioVersionService(NuclioClientService) {
        return {
            getVersion: getVersion
        };

        //
        // Public methods
        //

        /**
         * Send request to get current Nuclio's version
         * @returns {Promise} promise which will be resolved with all needed data regarding Nuclio's version
         */
        function getVersion() {
            return NuclioClientService.makeRequest(
                {
                    method: 'GET',
                    url: NuclioClientService.buildUrlWithPath('versions', ''),
                    withCredentials: false
                });
        }
    }
}());

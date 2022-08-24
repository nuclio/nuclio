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

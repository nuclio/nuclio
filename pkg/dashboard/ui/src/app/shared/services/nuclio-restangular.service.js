/*
Copyright 2023 The Nuclio Authors.

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
        .factory('NuclioRestangular', NuclioRestangular);

    // components bundled from iguazio.dashboard-controls (e.g. ExecutionLogsDataService) expect a
    // Restangular instance named "NuclioRestangular", configured against this app's own API base URL
    function NuclioRestangular(Restangular, lodash, ConfigService, NuclioNamespacesDataService) {
        return Restangular.withConfig(function (RestangularConfigurer) {
            RestangularConfigurer.setBaseUrl(lodash.trimEnd(ConfigService.url.nuclio.baseUrl, ' /'));

            // read the currently selected namespace on every request (not just once at factory
            // creation), so it stays correct across namespace switches - mirrors the header the
            // other *DataService's attach via NuclioNamespacesDataService.getNamespaceHeader()
            RestangularConfigurer.setFullRequestInterceptor(function (element, operation, route, url, headers, params, httpConfig) {
                return {
                    element: element,
                    headers: lodash.assign({}, headers, NuclioNamespacesDataService.getNamespaceHeader('x-nuclio-function-namespace')),
                    params: params,
                    httpConfig: httpConfig
                };
            });
        });
    }
}());

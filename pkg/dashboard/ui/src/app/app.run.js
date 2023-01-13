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
        .run(appInit);

    function appInit($urlRouter, $http, $injector, $window, i18next, lodash, ConfigService,
                     NuclioProjectsDataService) {
        // @if !IGZ_TESTING
        // @if IGZ_E2E_TESTING
        if ($injector.has('$httpBackend')) {
            var httpBackend = $injector.get('$httpBackend');
            httpBackend.whenGET(/dashboard-config\.json$/).passThrough();
        }

        // @endif
        $http
            .get('/dashboard-config.json', {
                responseType: 'json',
                headers: {'Cache-Control': 'no-cache'}
            })
            .then(function (config) {
                lodash.merge(ConfigService, config.data);
            })
            .then(function () {
                NuclioProjectsDataService.getFrontendSpec()
                    .then(function (response) {
                        lodash.assign(ConfigService.nuclio, {
                            allowedAuthenticationModes: lodash.get(response, 'allowedAuthenticationModes', []),
                            autoScaleMetrics: lodash.get(response, 'autoScaleMetrics', {}),
                            defaultFunctionConfig: lodash.get(response, 'defaultFunctionConfig', {}),
                            defaultFunctionPodResources: lodash.get(response, 'defaultFunctionPodResources', {}),
                            externalIPAddress: lodash.get(response, 'externalIPAddresses[0]', ''),
                            imageNamePrefixTemplate: lodash.get(response, 'imageNamePrefixTemplate', ''),
                            ingressHostTemplate: lodash.get(response, 'defaultHTTPIngressHostTemplate', ''),
                            namespace: lodash.get(response, 'namespace', ''),
                            platformKind: lodash.get(response, 'platformKind', ''),
                            scaleToZero: lodash.get(response, 'scaleToZero', {}),
                            validFunctionPriorityClassNames: lodash.get(response, 'validFunctionPriorityClassNames', null)
                        });
                    });
            })
            .then(function () {
                $urlRouter.listen();
                $urlRouter.sync();
            });
        // @endif

        /*eslint angular/on-watch: 0*/

        i18next
            .use($window.i18nextChainedBackend)
            .use($window.i18nextBrowserLanguageDetector);

        i18next.init({
            debug: false,
            fallbackLng: 'en',
            preload: ['en'],
            initImmediate: false,
            nonExplicitWhitelist: true,
            partialBundledLanguages: true,
            defaultNs: 'common',
            ns: [
                'common',
                'functions'
            ],
            // @if !IGZ_TESTING
            backend: {
                backends: [
                    $window.i18nextLocalStorageBackend,
                    $window.i18nextXHRBackend
                ],
                backendOptions: [
                    {
                        expirationTime: ConfigService.i18nextExpirationTime,
                        defaultVersion: 'v0.5'
                    },
                    {
                        loadPath: 'assets/i18n/{{lng}}/{{ns}}.json',
                        customHeaders: {
                            'Cache-Control': 'no-cache'
                        },
                    }
                ]
            }
            // @endif
        });
    }
}());

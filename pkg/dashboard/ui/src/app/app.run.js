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
                            externalIPAddress: lodash.get(response, 'externalIPAddresses[0]', ''),
                            ingressHostTemplate: lodash.get(response, 'defaultHTTPIngressHostTemplate', '')
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
                'header',
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
                        expirationTime: 24 * 60 * 60 * 1000
                    },
                    {
                        loadPath: 'assets/i18n/{{lng}}/{{ns}}.json'
                    }
                ]
            }
            // @endif
        });
    }
}());

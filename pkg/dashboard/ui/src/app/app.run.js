(function () {
    'use strict';

    angular.module('nuclio.app')
        .run(appInit);

    function appInit($urlRouter, $http, $injector, lodash, ConfigService, NuclioProjectsDataService) {
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
                NuclioProjectsDataService.getExternalIPAddresses()
                    .then(function (response) {
                        ConfigService.externalIPAddress = response.externalIPAddresses.addresses[0];
                    })
                    .catch(function () {
                        ConfigService.externalIPAddress = null;
                    });
            })
            .then(function () {
                $urlRouter.listen();
                $urlRouter.sync();
            });
        // @endif

        /*eslint angular/on-watch: 0*/
    }
}());

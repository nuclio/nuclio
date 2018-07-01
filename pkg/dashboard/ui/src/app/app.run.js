(function () {
    'use strict';

    angular.module('nuclio.app')
        .run(appInit);

    function appInit($rootScope, $state, $urlRouter, $http, $httpBackend, $injector, lodash, ConfigService, DialogsService, NuclioProjectsDataService) {
        // @if !IGZ_TESTING
        $rootScope.$on('$locationChangeSuccess', function (event) {
            // @if IGZ_E2E_TESTING
            if ($injector.has('$httpBackend')) {
                var httpBackend = $injector.get('$httpBackend');
                httpBackend.whenGET(/dashboard-config\.json$/).passThrough();
            }

            // @endif
            if ($state.current.name === '') {
                event.preventDefault();
                $http
                    .get('/dashboard-config.json', {
                        responseType: 'json',
                        headers: {'Cache-Control': 'no-cache'}
                    })
                    .then(function (config) {
                        lodash.merge(ConfigService, config.data);
                        $urlRouter.sync();
                    })
                    .then(function () {
                        NuclioProjectsDataService.getExternalIPAddresses()
                            .then(function (response) {
                                ConfigService.externalIPAddress = response.data.externalIPAddresses.addresses[0];
                            })
                            .catch(function () {
                                DialogsService.alert('Oops: Unknown error occurred while retrieving external IP address');
                            });
                    });
            }
        });
        // @endif

        /*eslint angular/on-watch: 0*/
        $urlRouter.listen();
    }
}());

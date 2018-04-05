(function () {
    'use strict';

    angular.module('nuclio.app')
        .run(appInit);

    function appInit($rootScope, $urlRouter, $http, $httpBackend, $injector, lodash, ConfigService) {
        // @if !IGZ_TESTING
        $rootScope.$on('$locationChangeSuccess', function (event) {
            // @if IGZ_E2E_TESTING
            if ($injector.has('$httpBackend')) {
                var httpBackend = $injector.get('$httpBackend');
                httpBackend.whenGET(/dashboard-config\.json$/).passThrough();
            }

            // @endif
            event.preventDefault();
            $http
                .get('/dashboard-config.json', {
                    responseType: 'json',
                    headers: {'Cache-Control': 'no-cache'}
                })
                .then(function (config) {
                    lodash.merge(ConfigService, config.data);
                    $urlRouter.sync();
                });
        });
        // @endif

        /*eslint angular/on-watch: 0*/
        $urlRouter.listen();
    }
}());

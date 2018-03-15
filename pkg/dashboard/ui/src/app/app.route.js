(function () {
    'use strict';

    angular.module('iguazio.app')
        .config(routes);

    function routes($stateProvider, $urlRouterProvider) {
        $urlRouterProvider.deferIntercept();

        $stateProvider.state('default', {
            url: '/',
            template: '<igz-default></igz-default>'
        });

        $urlRouterProvider.otherwise(function ($injector) {
            $injector.get('$state').go('default');
        });
    }
}());

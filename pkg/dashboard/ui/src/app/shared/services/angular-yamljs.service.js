angular.module('angular-yamljs', [])
    .provider('YAML', function () {
        this.$get = ['$window', function ($window) {
            return $window.YAML;
        }];
    });

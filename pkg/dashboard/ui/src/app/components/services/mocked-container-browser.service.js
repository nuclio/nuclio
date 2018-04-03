(function () {
    'use strict';

    angular.module('iguazio.app')
        .factory('ContainerBrowserService', ContainerBrowserService);

    function ContainerBrowserService() {
        return {
            onFilesDropped: angular.noop
        };
    }
}());

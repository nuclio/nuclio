(function () {
    'use strict';

    angular.module('iguazio.test.e2e')
        .factory('MockBackendService', MockBackendService);

    // injects mock objects and return them for requests from e2e tests
    MockBackendService.$inject = ['$httpBackend'];
    function MockBackendService($httpBackend) {
        var injectedMocksMap = {};

        return {
            injectMocks: injectMocks
        };

        // injects an array of specific mock objects
        function injectMocks(mocks) {
            angular.forEach(mocks, injectMock);
        }

        //
        // private methods
        //

        // injects a specific mock object
        function injectMock(mock) {
            var key = mock.request.method + '_' + mock.request.url;

            if (_.has(mock.request, 'data')) {
                key += '_' + JSON.stringify(mock.request.data)
            }

            // check whether the injectedMocksMap contains given mock
            if (angular.isUndefined(injectedMocksMap[key])) {
                // inject mock
                injectedMocksMap[key] = $httpBackend
                    .when(mock.request.method, mock.request.url, mock.request.data, mock.request.headers)
                    .respond(mock.response.status, mock.response.data, mock.response.headers);
            } else {
                // override mock response
                injectedMocksMap[key].respond(mock.response.status, mock.response.data, mock.response.headers);
            }
        }
    }
}());
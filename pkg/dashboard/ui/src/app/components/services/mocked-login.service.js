(function () {
    'use strict';

    angular.module('iguazio.app')
        .factory('LoginService', LoginService);

    function LoginService() {

        var service = {
            hasCapabilities: hasCapabilities
        };

        return service; // using this instead of returning the object directly - for testability (unit-tests can change
                        // behavior with: `spyOn(LoginService, 'methodName').and.callFake(...)` if the service method
                        // uses `service.methodName()` instead of just `methodName()`)

        //
        // Public methods
        //

        /**
         * Tests whether or not one or more capabilities are all on the stored capability list of the current session
         * @param {string|Array.<string>} capabilities - the capability to test, or a list of capabilities to test
         * @returns {boolean} `true`
         */
        function hasCapabilities(capabilities) {
            return true;
        }
    }
}());

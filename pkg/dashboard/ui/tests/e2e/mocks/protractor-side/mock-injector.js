module.exports = (function () {
    return {
        injectMocks: injectMocks
    };

    // injects specific mock object
    function injectMocks(done, mocks) {
        var stringToInject = JSON.stringify(mocks);

        // execute script in loaded browser page and call Angular service for mock injections
        browser.executeAsyncScript(function (mocks, callback) {
            var mocksToInject = JSON.parse(mocks);

            // get an injector
            var injector = angular.element('html').injector();

            // get a service
            var MockBackendService = injector.get('MockBackendService');

            // convert string that wrapped in '/' to RegExp
            mocksToInject.forEach(function (mock) {
                if (mock.request.url[0] === '/') {
                    mock.request.url = new RegExp(mock.request.url.slice(1, mock.request.url.length - 1));
                }
            });

            // call a service method to inject mocks
            MockBackendService.injectMocks(mocksToInject);

            callback();
        }, stringToInject)
            .then(function () {

                // call done function to announce that asynchronous function is executed
                done();
            });
    }
})();
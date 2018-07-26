(function () {
    'use strict';

    angular.module('nuclio.app')
        .factory('NuclioEventService', NuclioEventService);

    function NuclioEventService(lodash, NuclioClientService) {
        return {
            deleteEvent: deleteEvent,
            deployEvent: deployEvent,
            getEvents: getEvents,
            invokeFunction: invokeFunction
        };

        //
        // Public methods
        //

        /**
         * Sends request to delete event
         */
        function deleteEvent(eventData) {
            var headers = {
                'Content-Type': 'application/json'
            };

            var config = {
                data: eventData,
                method: 'delete',
                headers: headers,
                withCredentials: false,
                url: NuclioClientService.buildUrlWithPath('function_events')
            };

            return NuclioClientService.makeRequest(config);
        }

        /**
         * Sends request to deploy event
         * @param {Object} eventData - object with all needed data to deploy event
         * @param {boolean} isNewEvent - represents state of event (new event, or existing event)
         */
        function deployEvent(eventData, isNewEvent) {

            // check if it's a new event.
            // If yes, then send POST request, otherwise it's a update of existing event, so send PUT request
            var method = isNewEvent ? 'post' : 'put';
            var headers = {
                'Content-Type': 'application/json'
            };

            var config = {
                data: eventData,
                method: method,
                headers: headers,
                withCredentials: false,
                url: NuclioClientService.buildUrlWithPath('function_events')
            };

            return NuclioClientService.makeRequest(config);
        }

        /**
         * Gets list of events
         * @param {Object} functionData - object with all needed data to get events list
         */
        function getEvents(functionData) {
            var headers = {
                'x-nuclio-function-name': functionData.metadata.name
            };

            var config = {
                method: 'get',
                headers: headers,
                withCredentials: false,
                url: NuclioClientService.buildUrlWithPath('function_events')
            };

            return NuclioClientService.makeRequest(config)
                .then(function (response) {
                    return response.data;
                });
        }

        /**
         * Invokes the function
         * @param {Object} eventData
         * @param {Promise} canceller
         * @returns {Promise}
         */
        function invokeFunction(eventData, canceller) {
            var headers = {
                'x-nuclio-function-name': eventData.metadata.labels['nuclio.io/function-name'],
                'x-nuclio-function-namespace': eventData.metadata.namespace,
                'x-nuclio-invoke-via': 'external-ip',
                'x-nuclio-path': eventData.spec.attributes.path
            };
            headers = lodash.merge(headers, eventData.spec.attributes.headers);

            var config = {
                data: eventData.spec.body,
                method: eventData.spec.attributes.method,
                headers: headers,
                url: NuclioClientService.buildUrlWithPath('function_invocations')
            };
            if (angular.isDefined(canceller)) {
                config.timeout = canceller.promise;
            }

            return NuclioClientService.makeRequest(config)
                .then(parseResult, parseResult);

            function parseResult(result) {
                return {
                    status: result.status,
                    statusText: result.statusText,
                    headers: result.headers(),
                    body: result.data
                };
            }
        }
    }
}());

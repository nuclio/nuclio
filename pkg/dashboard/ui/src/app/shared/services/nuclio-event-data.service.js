/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

(function () {
    'use strict';

    angular.module('nuclio.app')
        .factory('NuclioEventDataService', NuclioEventDataService);

    function NuclioEventDataService(lodash, NuclioClientService, NuclioNamespacesDataService) {
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
            var namespace = NuclioNamespacesDataService.getNamespace();

            if (lodash.isNil(namespace)) {
                eventData.metadata = lodash.omit(eventData.metadata, 'namespace');
            }

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
            var namespace = NuclioNamespacesDataService.getNamespace();

            if (lodash.isNil(namespace)) {
                eventData.metadata = lodash.omit(eventData.metadata, 'namespace');
            }

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

            lodash.assign(headers, NuclioNamespacesDataService.getNamespaceHeader('x-nuclio-function-event-namespace'));

            var config = {
                method: 'get',
                headers: headers,
                withCredentials: false,
                url: NuclioClientService.buildUrlWithPath('function_events')
            };

            return NuclioClientService.makeRequest(config);
        }

        /**
         * Invokes the function.
         * @param {Object} eventData - the function event to invoke function with.
         * @param {boolean} skipTlsVerification - if true should add "skip tls verification" header.
         * @param {string} [invokeUrl] - the invocation URL to use (could be internal or external).
         * @param {Promise} [canceler] - if provided, function invocation is canceled on resolving this promise.
         * @returns {Promise}
         */
        function invokeFunction(eventData, skipTlsVerification, invokeUrl, canceler) {
            var userDefinedHeaders = lodash.get(eventData, 'spec.attributes.headers', {});
            var headers = lodash.assign({}, userDefinedHeaders, {
                'x-nuclio-function-name': eventData.metadata.labels['nuclio.io/function-name'],
                'x-nuclio-invoke-url': invokeUrl,
                'x-nuclio-path': eventData.spec.attributes.path,
                'x-nuclio-log-level': eventData.spec.attributes.logLevel,
                'x-nuclio-skip-tls-verification': skipTlsVerification
            });

            lodash.assign(headers, NuclioNamespacesDataService.getNamespaceHeader('x-nuclio-function-namespace'));

            var config = {
                data: eventData.spec.body,
                method: eventData.spec.attributes.method,
                headers: headers,
                timeout: lodash.defaultTo(canceler, null),
                url: NuclioClientService.buildUrlWithPath('function_invocations')
            };

            return NuclioClientService.makeRequest(config, false)
                .then(parseResult, parseResult);

            function parseResult(result) {
                return lodash.isError(result) ? {
                    status: -1,
                    statusText: 'Invalid response',
                    headers: { 'Content-Type': 'text/plain' },
                    body: result.message
                } : {
                    status: result.status,
                    statusText: result.statusText,
                    headers: result.headers(),
                    body: result.data
                };
            }
        }
    }
}());

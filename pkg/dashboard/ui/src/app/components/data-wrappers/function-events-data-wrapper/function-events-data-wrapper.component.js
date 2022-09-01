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
        .component('functionEventsDataWrapper', {
            bindings: {
                version: '<'
            },
            templateUrl: 'data-wrappers/function-events-data-wrapper/function-events-data-wrapper.tpl.html',
            controller: FunctionEventsDataWrapperController
        });

    function FunctionEventsDataWrapperController(NuclioEventDataService) {
        var ctrl = this;

        ctrl.createFunctionEvent = createFunctionEvent;
        ctrl.deleteFunctionEvent = deleteFunctionEvent;
        ctrl.getFunctionEvents = getFunctionEvents;
        ctrl.invokeFunction = invokeFunction;

        //
        // Public methods
        //

        /**
         * Creates new function event
         * @param {Object} eventData
         * @param {boolean} isNewEvent
         * @returns {Promise}
         */
        function createFunctionEvent(eventData, isNewEvent) {
            return NuclioEventDataService.deployEvent(eventData, isNewEvent);
        }

        /**
         * Deletes function event
         * @param {Object} eventData
         * @returns {Promise}
         */
        function deleteFunctionEvent(eventData) {
            return NuclioEventDataService.deleteEvent(eventData);
        }

        /**
         * Gets list of events
         * @param {Object} functionData
         * @returns {Promise}
         */
        function getFunctionEvents(functionData) {
            return NuclioEventDataService.getEvents(functionData);
        }

        /**
         * Invoke function event
         * @param {Object} eventData
         * @param {string} invokeUrl
         * @param {Promise} canceller
         * @returns {Promise}
         */
        function invokeFunction(eventData, invokeUrl, canceller) {
            return NuclioEventDataService.invokeFunction(eventData, invokeUrl, canceller);
        }
    }
}());

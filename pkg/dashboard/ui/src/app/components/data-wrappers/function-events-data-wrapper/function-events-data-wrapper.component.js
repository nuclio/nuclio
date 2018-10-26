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

    function FunctionEventsDataWrapperController(NuclioEventService, NuclioProjectsDataService) {
        var ctrl = this;

        ctrl.createFunctionEvent = createFunctionEvent;
        ctrl.deleteFunctionEvent = deleteFunctionEvent;
        ctrl.getExternalIPAddresses = getExternalIPAddresses;
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
            return NuclioEventService.deployEvent(eventData, isNewEvent);
        }

        /**
         * Deletes function event
         * @param {Object} eventData
         * @returns {Promise}
         */
        function deleteFunctionEvent(eventData) {
            return NuclioEventService.deleteEvent(eventData);
        }

        /**
         * Gets external IP addresses
         * @returns {Promise}
         */
        function getExternalIPAddresses() {
            return NuclioProjectsDataService.getExternalIPAddresses();
        }

        /**
         * Gets list of events
         * @param {Object} functionData
         * @returns {Promise}
         */
        function getFunctionEvents(functionData) {
            return NuclioEventService.getEvents(functionData);
        }

        /**
         * Invoke function event
         * @param {Object} eventData
         * @param {Promise} canceller
         * @returns {Promise}
         */
        function invokeFunction(eventData, canceller) {
            return NuclioEventService.invokeFunction(eventData, canceller);
        }
    }
}());

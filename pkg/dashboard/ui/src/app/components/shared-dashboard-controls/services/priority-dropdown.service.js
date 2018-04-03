(function () {
    'use strict';

    angular.module('iguazio.app')
        .factory('PriorityDropdownService', PriorityDropdownService);

    function PriorityDropdownService() {
        return {
            getName: getName,
            getPrioritiesArray: getPrioritiesArray
        };

        //
        // Public methods
        //

        /**
         * Gets array of priority types
         * @returns {Array}
         */
        function getPrioritiesArray() {
            return [
                {
                    name: 'Real-time',
                    type: 'realtime',
                    icon: {
                        name: 'igz-icon-priority-realtime'
                    }
                },
                {
                    name: 'High',
                    type: 'high',
                    icon: {
                        name: 'igz-icon-priority-high'
                    }
                },
                {
                    name: 'Standard',
                    type: 'standard',
                    icon: {
                        name: 'igz-icon-priority-standard'
                    }
                },
                {
                    name: 'Low',
                    type: 'low',
                    icon: {
                        name: 'igz-icon-priority-low'
                    }
                }
            ];
        }

        /**
         * Gets name of priority depends on type
         * @param {string} type
         * @returns {string}
         */
        function getName(type) {
            return type === 'realtime' ? 'Real-time' :
                   type === 'high'     ? 'High'      :
                   type === 'standard' ? 'Standard'  :
                   type === 'low'      ? 'Low'       : '';
        }
    }
}());

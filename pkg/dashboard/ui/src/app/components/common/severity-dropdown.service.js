(function () {
    'use strict';

    angular.module('iguazio.app')
        .factory('SeverityDropdownService', SeverityDropdownService);

    function SeverityDropdownService() {
        return {
            getSeveritiesArray: getSeveritiesArray
        };

        //
        // Public methods
        //

        /**
         * Gets array of severity types
         * @returns {Array}
         */
        function getSeveritiesArray() {
            return [
                {
                    name: 'Error',
                    type: 'error',
                    icon: {
                        name: 'igz-icon-warning severity-icon critical'
                    }
                },
                {
                    name: 'Debug',
                    type: 'debug',
                    icon: {
                        name: 'igz-icon-warning severity-icon major'
                    }
                },
                {
                    name: 'Warning',
                    type: 'warning',
                    icon: {
                        name: 'igz-icon-warning severity-icon warning'
                    }
                },
                {
                    name: 'Info',
                    type: 'info',
                    icon: {
                        name: 'igz-icon-info-round severity-icon info'
                    }
                }
            ];
        }
    }
}());

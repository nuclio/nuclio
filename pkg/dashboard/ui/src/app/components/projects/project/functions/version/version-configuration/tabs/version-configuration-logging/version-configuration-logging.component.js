(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclVersionConfigurationLogging', {
            bindings: {
                version: '<'
            },
            templateUrl: 'projects/project/functions/version/version-configuration/tabs/version-configuration-logging/version-configuration-logging.tpl.html',
            controller: NclVersionConfigurationLoggingController
        });

    function NclVersionConfigurationLoggingController(lodash) {
        var ctrl = this;

        ctrl.inputValueCallback = inputValueCallback;
        ctrl.setPriority = setPriority;

        //
        // Public methods
        //

        /**
         * Update data callback
         * @param {string} newData
         * @param {string} field
         */
        function inputValueCallback(newData, field) {
            lodash.set(ctrl.version, field, newData);
        }

        /**
         * Sets logger level
         * @param {Object} item
         */
        function setPriority(item) {
            lodash.set(ctrl.version, 'spec.loggerSinks.level', item.type);
        }
    }
}());

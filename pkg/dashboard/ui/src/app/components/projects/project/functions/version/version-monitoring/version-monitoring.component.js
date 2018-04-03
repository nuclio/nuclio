(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclEditVersionMonitoring', {
            templateUrl: 'projects/project/functions/version/version-monitoring/version-monitoring.tpl.html',
            controller: NclVersionMonitoringController
        });

    function NclVersionMonitoringController() {
        var ctrl = this;

        ctrl.$onInit = onInit;

        //
        // Hook methods
        //

        /**
         * Initialization method
         */
        function onInit() {
        }

        //
        // Public methods
        //

        //
        // Private method
        //
    }
}());

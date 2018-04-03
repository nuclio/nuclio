(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclVersionConfigurationBuild', {
            templateUrl: 'projects/project/functions/version/version-configuration/tabs/version-configuration-build/version-configuration-build.tpl.html',
            controller: NclVersionConfigurationBuildController
        });

    function NclVersionConfigurationBuildController(lodash) {
        var ctrl = this;

        ctrl.$onInit = onInit;

        ctrl.inputValueCallback = inputValueCallback;

        //
        // Hook methods
        //

        /**
         * Initialization method
         */
        function onInit() {
            lodash.defaultsDeep(ctrl.version, {
                spec: {
                    build: {}
                }
            });
        }

        //
        // Public methods
        //

        /**
         * Update data callback
         * @param {string} newData
         * @param {string} field
         */
        function inputValueCallback(newData, field) {
            lodash.set(ctrl.version, 'build.' + field, newData);
        }
    }
}());

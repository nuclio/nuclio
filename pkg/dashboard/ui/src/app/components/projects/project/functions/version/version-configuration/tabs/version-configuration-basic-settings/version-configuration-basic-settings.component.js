(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclVersionConfigurationBasicSettings', {
            bindings: {
                version: '<'
            },
            templateUrl: 'projects/project/functions/version/version-configuration/tabs/version-configuration-basic-settings/version-configuration-basic-settings.tpl.html',
            controller: NclVersionConfigurationBasicSettingsController
        });

    function NclVersionConfigurationBasicSettingsController(lodash) {
        var ctrl = this;

        ctrl.enableEdit = false;
        ctrl.enableTimeout = false;
        ctrl.timeout = {
            min: 0,
            sec: 0
        };

        ctrl.$onInit = onInit;

        ctrl.inputValueCallback = inputValueCallback;

        //
        // Hook methods
        //

        /**
         * Initialization method
         */
        function onInit() {
            var timeoutSeconds = lodash.get(ctrl.version, 'spec.timeoutSeconds');

            if (lodash.isNumber(timeoutSeconds)) {
                ctrl.timeout.min = Math.floor(timeoutSeconds / 60);
                ctrl.timeout.sec = Math.floor(timeoutSeconds % 60);
            }
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
            if (lodash.includes(field, 'timeout')) {
                lodash.set(ctrl, field, Number(newData));

                lodash.set(ctrl.version, 'spec.timeoutSeconds', ctrl.timeout.min * 60 + ctrl.timeout.sec);
            } else {
                lodash.set(ctrl.version, field, newData);
            }
        }
    }
}());

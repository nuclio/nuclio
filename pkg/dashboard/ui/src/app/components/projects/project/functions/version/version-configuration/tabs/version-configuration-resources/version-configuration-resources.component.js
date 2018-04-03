(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclVersionConfigurationResources', {
            bindings: {
                version: '<'
            },
            templateUrl: 'projects/project/functions/version/version-configuration/tabs/version-configuration-resources/version-configuration-resources.tpl.html',
            controller: NclVersionConfigurationResourcesController
        });

    function NclVersionConfigurationResourcesController(lodash) {
        var ctrl = this;

        ctrl.$onInit = onInit;

        ctrl.numberInputCallback = numberInputCallback;

        //
        // Hook methods
        //

        /**
         * Initialization method
         */
        function onInit() {
            ctrl.memorySliderConfig = {
                name: 'Memory',
                value: 0,
                valueLabel: '',
                pow: 0,
                unitLabel: 'MB',
                labelHelpIcon: false,
                options: {
                    floor: 1,
                    id: 'memory',
                    ceil: 1025,
                    showSelectionBar: false,
                    onChange: null,
                    onEnd: null
                }
            };
            ctrl.cpuSliderConfig = {
                name: 'CPU',
                value: 0,
                valueLabel: '',
                pow: 0,
                unitLabel: '',
                labelHelpIcon: false,
                options: {
                    floor: 1,
                    id: 'cpu',
                    ceil: 10,
                    showSelectionBar: false,
                    onChange: null,
                    onEnd: null
                }
            };
            ctrl.defaultMeasureUnits = [
                {
                    pow: 1,
                    name: 'KB'
                },
                {
                    pow: 2,
                    name: 'MB'
                },
                {
                    pow: 3,
                    name: 'GB'
                }
            ];
        }

        //
        // Public methods
        //

        /**
         * Update data callback
         * @param {string} newData
         * @param {string} field
         */
        function numberInputCallback(newData, field) {
            if (ctrl.resourcesForm.$valid) {
                lodash.set(ctrl.version, field, newData);
            }
        }
    }
}());

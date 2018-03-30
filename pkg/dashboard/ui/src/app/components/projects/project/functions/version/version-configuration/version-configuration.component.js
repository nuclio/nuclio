(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclVersionConfiguration', {
            bindings: {
                version: '<'
            },
            templateUrl: 'projects/project/functions/version/version-configuration/version-configuration.tpl.html',
            controller: NclVersionConfigurationController
        });

    function NclVersionConfigurationController(lodash) {
        var ctrl = this;

        ctrl.scrollConfig = {
            axis: 'y',
            advanced: {
                autoScrollOnFocus: false,
                updateOnContentResize: true
            }
        };

        ctrl.$onInit = onInit;

        //
        // Hook methods
        //

        /**
         * Initialization method
         */
        function onInit() {
            if (angular.isUndefined(ctrl.version)) {
                ctrl.version = {};
            }

            lodash.defaultsDeep(ctrl.version, {
                metadata: {
                    name: '',
                    namespace: '',
                    labels: {},
                    annotations: {}
                },
                spec: {
                    description: '',
                    timeoutSeconds: 0,
                    runtime: '',
                    env: [],
                    loggerSinks: {
                        level: 'debug',
                        sink: ''
                    },
                    minReplicas: 0,
                    maxReplicas: 1
                }
            });
        }
    }
}());

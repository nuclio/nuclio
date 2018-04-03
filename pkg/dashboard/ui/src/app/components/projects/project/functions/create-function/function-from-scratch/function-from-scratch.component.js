(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclFunctionFromScratch', {
            bindings: {
                toggleSplashScreen: '&'
            },
            templateUrl: 'projects/project/functions/create-function/function-from-scratch/function-from-scratch.tpl.html',
            controller: FunctionFromScratchController
        });

    function FunctionFromScratchController($interval, $state, $stateParams, lodash, DialogsService, NuclioFunctionsDataService,
                                           NuclioProjectsDataService, ValidatingPatternsService) {
        var ctrl = this;
        var interval = null;
        var namespace = '';

        ctrl.functionData = {};
        ctrl.project = {};
        ctrl.runtimes = [];
        ctrl.selectedRuntime = null;

        ctrl.$onInit = onInit;

        ctrl.validationPatterns = ValidatingPatternsService;

        ctrl.createFunction = createFunction;
        ctrl.inputValueCallback = inputValueCallback;
        ctrl.onDropdownDataChange = onDropdownDataChange;

        /**
         * Initialization method
         */
        function onInit() {
            ctrl.toggleSplashScreen({value: true});

            // get project to get know his namespace
            NuclioProjectsDataService.getProject($stateParams.projectId)
                .then(function (project) {
                    namespace = project.metadata.namespace;
                    ctrl.runtimes = getRuntimes();
                    ctrl.selectedRuntime = getDefaultRuntime();

                    initFunctionData();

                    ctrl.toggleSplashScreen({value: false});
                })
                .catch(function () {
                    DialogsService.alert('Oops: Unknown error occurred');
                });
        }

        //
        // Public methods
        //

        /**
         * Callback handler for 'create function' button
         * Creates function with defined data.
         */
        function createFunction() {

            // create function only when form is valid
            if (ctrl.functionFromScratchForm.$valid) {
                NuclioFunctionsDataService.createFunction(ctrl.functionData)
                    .then(function () {
                        ctrl.toggleSplashScreen({value: true});

                        pullFunctionState();
                    })
                    .catch(function () {
                        DialogsService.alert('Oops: Unknown error occurred');
                    })
            }
        }

        /**
         * Set data returned by validating input component
         * @param {string} data - data to be set
         * @param {string} field - field which should be filled
         */
        function inputValueCallback(data, field) {
            if (!lodash.isNil(data)) {
                lodash.set(ctrl, 'functionData.metadata.' + field, data);
            }
        }

        /**
         * Set data returned by default dropdown component
         * @param {Object} item - the new data
         * @param {boolean} isItemChanged - was value changed or not
         */
        function onDropdownDataChange(item, isItemChanged) {
            if (!lodash.isNil(item) && isItemChanged) {
                lodash.assign(ctrl.functionData.spec, {
                    runtime: item.id,
                    handler: item.id === 'golang' ? 'main:Handler' : 'main:handler',
                    build: {
                        functionSourceCode: item.sourceCode
                    }
                });
            }
        }

        //
        // Private methods
        //

        /**
         * Gets all runtimes
         * @returns {Array}
         */
        function getRuntimes() {
            return [
                {
                    id: 'golang',
                    name: 'Golang',
                    sourceCode: 'cGFja2FnZSBtYWluDQoNCmltcG9ydCAoDQogICAgImdpdGh1Yi5jb20vbnVjbGlvL251Y2xpby1zZGstZ28iDQo' +
                    'pDQoNCmZ1bmMgSGFuZGxlcihjb250ZXh0ICpudWNsaW8uQ29udGV4dCwgZXZlbnQgbnVjbGlvLkV2ZW50KSAoaW50ZXJmYWNle3' +
                    '0sIGVycm9yKSB7DQogICAgcmV0dXJuIG5pbCwgbmlsDQp9', // source code in base64
                    visible: true
                },
                {
                    id: 'python',
                    name: 'Python',
                    sourceCode: 'ZGVmIGhhbmRsZXIoY29udGV4dCwgZXZlbnQpOg0KICAgIHBhc3M=', // source code in base64
                    visible: true
                },
                {
                    id: 'pypy',
                    name: 'PyPy',
                    sourceCode: 'ZGVmIGhhbmRsZXIoY29udGV4dCwgZXZlbnQpOg0KICAgIHBhc3M=', // source code in base64
                    visible: true
                },
                {
                    id: 'nodejs',
                    sourceCode: 'ZXhwb3J0cy5oYW5kbGVyID0gZnVuY3Rpb24oY29udGV4dCwgZXZlbnQpIHsNCn07', // source code in base64
                    name: 'NodeJS',
                    visible: true
                },
                {
                    id: 'shell',
                    name: 'Shell Java',
                    sourceCode: '',
                    visible: true
                }
            ];
        }

        /**
         * Gets default runtime
         * @returns {object} default runtime
         */
        function getDefaultRuntime() {
            return lodash.find(ctrl.runtimes, ['id', 'golang']);
        }

        /**
         * Initialize object for function from scratch
         */
        function initFunctionData() {
            ctrl.functionData = {
                metadata: {
                    name: '',
                    namespace: namespace
                },
                spec: {
                    handler: namespace === 'golang' ? 'main:Handler' : 'main:handler',
                    runtime: ctrl.selectedRuntime.id,
                    build: {
                        functionSourceCode: ctrl.selectedRuntime.sourceCode
                    }
                }
            };
        }

        /**
         * Pulls function status.
         * Periodically sends request to get function's state, until state will not be 'ready' or 'error'
         */
        function pullFunctionState() {
            interval = $interval(function () {
                NuclioFunctionsDataService.getFunction(ctrl.functionData.metadata)
                    .then(function (response) {
                        if (lodash.includes(['ready', 'error'], response.status.state)) {
                            if (!lodash.isNil(interval)) {
                                $interval.cancel(interval);
                                interval = null;
                            }

                            ctrl.toggleSplashScreen({value: false});

                            $state.go('app.project.functions');
                        }
                    })
                    .catch(function (error) {
                        if (error.status !== 404) {
                            if (!lodash.isNil(interval)) {
                                $interval.cancel(interval);
                                interval = null;
                            }

                            ctrl.toggleSplashScreen({value: false});
                        }
                    });
            }, 2000);
        }
    }
}());

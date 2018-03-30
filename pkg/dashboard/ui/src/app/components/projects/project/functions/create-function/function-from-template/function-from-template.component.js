(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclFunctionFromTemplate', {
            bindings: {
                toggleSplashScreen: '&'
            },
            templateUrl: 'projects/project/functions/create-function/function-from-template/function-from-template.tpl.html',
            controller: FunctionFromTemplateController
        });

    function FunctionFromTemplateController($interval, $state, $stateParams, $q, lodash, DialogsService, ValidatingPatternsService,
                                            NuclioFunctionsDataService, NuclioProjectsDataService) {
        var ctrl = this;
        var interval = null;
        var namespace = '';

        ctrl.functionData = {};
        ctrl.selectedTemplate = '';
        ctrl.templates = [];

        ctrl.$onInit = onInit;

        ctrl.validationPatterns = ValidatingPatternsService;

        ctrl.createFunction = createFunction;
        ctrl.inputValueCallback = inputValueCallback;
        ctrl.isTemplateSelected = isTemplateSelected;
        ctrl.selectTemplate = selectTemplate;

        //
        // Hook methods
        //

        /**
         * Initialization method
         */
        function onInit() {
            ctrl.toggleSplashScreen({value: true});

            // get project to get know his namespace
            NuclioProjectsDataService.getProject($stateParams.projectId)
                .then(function (project) {
                    namespace = project.metadata.namespace;

                    initFunctionData();
                })
                .catch(function () {
                    DialogsService.alert('Oops: Unknown error occurred');

                    ctrl.toggleSplashScreen({value: false});
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
            if (ctrl.functionFromTemplateForm.$valid && !lodash.isNil(ctrl.selectedTemplate)) {
                NuclioFunctionsDataService.createFunction(ctrl.functionData)
                    .then(function () {
                        ctrl.toggleSplashScreen({value: true});

                        pullFunctionState();
                    });
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
         * Checks which template type is selected.
         * Returns true if 'template' is equal to 'selectedTemplate'.
         * Which means that template from argument 'template' should be selected now.
         * @param {Object} templateName
         * @returns {boolean}
         */
        function isTemplateSelected(templateName) {
            return lodash.isEqual(templateName, ctrl.selectedTemplate);
        }

        /**
         * Selects template.
         * Sets new template as selected
         * @param {Object} templateName - template to be set
         */
        function selectTemplate(templateName) {
            if (!lodash.isEqual(templateName, ctrl.selectedTemplate)) {
                ctrl.selectedTemplate = templateName;

                lodash.set(ctrl, 'functionData.spec.runtime', ctrl.templates[ctrl.selectedTemplate].spec.runtime);
                lodash.set(ctrl, 'functionData.spec.build.functionSourceCode',
                    ctrl.templates[ctrl.selectedTemplate].spec.build.functionSourceCode);
            }
        }

        //
        // Private methods
        //

        /**
         * Gets default selected template
         * @returns {Object} template to be set as selected
         */
        function getSelectedTemplate() {
            return lodash.keys(ctrl.templates)[0];
        }

        /**
         * Initialize object for function from template
         */
        function initFunctionData() {

            // gets all available function templates
            NuclioFunctionsDataService.getTemplates()
                .then(function (repsonse) {
                    ctrl.templates = repsonse.data;
                    ctrl.selectedTemplate = getSelectedTemplate();
                    var selectedTemplate = ctrl.templates[ctrl.selectedTemplate];

                    ctrl.functionData = {
                        metadata: {
                            name: '',
                            namespace: namespace
                        },
                        spec: {
                            handler: namespace === 'golang' ? 'main:Handler' : 'main:handler',
                            runtime: selectedTemplate.spec.runtime,
                            build: {
                                functionSourceCode: selectedTemplate.spec.build.functionSourceCode
                            }
                        }
                    };
                })
                .catch(function () {
                    DialogsService.alert('Oops: Unknown error occurred');
                })
                .finally(function () {
                    ctrl.toggleSplashScreen({value: false});
                });
        }

        /**
         * Pulls function status.
         * Periodically sends request to get function's  state, until state will not be 'ready' or 'error'
         */
        function pullFunctionState() {
            interval = $interval(function () {
                NuclioFunctionsDataService.getFunction(ctrl.functionData.metadata)
                    .then(function (response) {
                        if (lodash.includes(['ready', 'error'], response.data.status.state)) {
                            if (!lodash.isNil(interval)) {
                                $interval.cancel(interval);

                                interval = null;
                            }

                            ctrl.toggleSplashScreen({value: false});
                        }
                    })
                    .catch(function (error) {
                        if (error.status !== 404) {
                            if (!lodash.isNil(interval)) {
                                $interval.cancel(interval);

                                interval = null;
                            }

                            ctrl.toggleSplashScreen({value: false});

                            $state.go('app.project.functions');
                        }
                    });
            }, 2000);
        }
    }
}());

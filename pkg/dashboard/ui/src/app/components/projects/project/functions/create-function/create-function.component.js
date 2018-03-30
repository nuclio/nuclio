(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclCreateFunction', {
            templateUrl: 'projects/project/functions/create-function/create-function.tpl.html',
            controller: CreateFunctionController
        });

    function CreateFunctionController($state, $stateParams, lodash, NuclioHeaderService, NuclioProjectsDataService) {
        var ctrl = this;
        var selectedFunctionType = 'from_scratch';

        ctrl.isSplashShowed = {
            value: false
        };
        ctrl.scrollConfig = {
            axis: 'yx',
            advanced: {
                updateOnContentResize: true
            }
        };

        ctrl.$onInit = onInit;

        ctrl.toggleSplashScreen = toggleSplashScreen;
        ctrl.isTypeSelected = isTypeSelected;
        ctrl.selectFunctionType = selectFunctionType;

        //
        // Hook methods
        //

        /**
         * Initialization method
         */
        function onInit() {
            NuclioProjectsDataService.getProject($stateParams.projectId)
                .then(function (project) {
                    ctrl.title = {
                        project: project.spec.displayName,
                        function: 'Create function'
                    };

                    NuclioHeaderService.updateMainHeader('Projects', ctrl.title, $state.current.name);
                });
        }

        //
        // Public methods
        //

        /**
         * Toggles splash screen.
         * If value is undefined then sets opposite itself's value, otherwise sets provided value.
         * @param {boolean} value - value to be set
         */
        function toggleSplashScreen(value) {
            lodash.defaultTo(value, !ctrl.isSplashShowed.value);
        }

        /**
         * Checks which function type is visible.
         * Returns true if 'functionType' is equal to 'selectedFunctionType'. Which means that function with type from
         * argument 'functionType' should be visible.
         * @param {string} functionType
         * @returns {boolean}
         */
        function isTypeSelected(functionType) {
            return lodash.isEqual(selectedFunctionType, functionType);
        }

        /**
         * Sets selected function type
         * @param {string} functionType
         */
        function selectFunctionType(functionType) {
            if (!lodash.isEqual(functionType, selectedFunctionType)) {
                selectedFunctionType = functionType;
            }
        }
    }
}());

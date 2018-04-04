(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclVersionConfigurationAnnotations', {
            bindings: {
                version: '<'
            },
            templateUrl: 'projects/project/functions/version/version-configuration/tabs/version-configuration-annotations/version-configuration-annotations.tpl.html',
            controller: NclVersionConfigurationAnnotationsController
        });

    function NclVersionConfigurationAnnotationsController($element, lodash, PreventDropdownCutOffService) {
        var ctrl = this;

        ctrl.scrollConfig = {
            axis: 'y',
            advanced: {
                updateOnContentResize: true
            }
        };

        ctrl.$onInit = onInit;
        ctrl.$postLink = postLink;

        ctrl.addNewAnnotation = addNewAnnotation;
        ctrl.handleAction = handleAction;
        ctrl.onChangeData = onChangeData;

        //
        // Hook methods
        //

        /**
         * Initialization method
         */
        function onInit() {
            var annotations =  lodash.get(ctrl.version, 'metadata.annotations', []);

            ctrl.annotations = lodash.map(annotations, function (value, key) {
                return {
                    name: key,
                    value: value
                };
            });
        }

        /**
         * Linking method
         */
        function postLink() {

            // Bind DOM-related preventDropdownCutOff method to component's controller
            PreventDropdownCutOffService.preventDropdownCutOff($element, '.three-dot-menu');
        }

        //
        // Public methods
        //

        /**
         * Adds new annotation
         */
        function addNewAnnotation() {
            ctrl.annotations.push({
                name: '',
                value: ''
            });
        }

        /**
         * Handler on specific action type
         * @param {string} actionType
         * @param {number} index - index of label in array
         */
        function handleAction(actionType, index) {
            if (actionType === 'delete') {
                ctrl.annotations.splice(index, 1);

                updateAnnotations();
            }
        }

        /**
         * Changes annotations data
         * @param {Object} label
         * @param {number} index
         */
        function onChangeData(label, index) {
            ctrl.annotations[index] = label;

            updateAnnotations();
        }

        //
        // Private methods
        //

        /**
         * Updates function`s labels
         */
        function updateAnnotations() {
            var newAnnotations = {};

            lodash.forEach(ctrl.annotations, function (annotation) {
                lodash.set(newAnnotations, annotation.name, annotation.value);
            });

            lodash.set(ctrl.version, 'metadata.annotations', newAnnotations);
        }
    }
}());

(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclVersionCode', {
            bindings: {
                version: '<'
            },
            templateUrl: 'projects/project/functions/version/version-code/version-code.tpl.html',
            controller: NclVersionCodeController
        });

    function NclVersionCodeController($element, $timeout, lodash, PreventDropdownCutOffService) {
        var ctrl = this;
        ctrl.codeEntryTypeArray = [
            {
                id: 'none',
                visible: true,
                name: 'Edit online'
            },
            {
                id: 'none',
                visible: true,
                name: 'Upload archive'
            },
            {
                id: 'none',
                visible: true,
                name: 'S3 URL'
            },
            {
                id: 'none',
                visible: true,
                name: 'Git repository'
            }
        ];

        ctrl.runtimeArray = [
            {
                id: 'none',
                visible: true,
                name: 'Golang 1.9'
            },
            {
                id: 'none',
                visible: true,
                name: 'Python 2.7'
            },
            {
                id: 'none',
                visible: true,
                name: 'Python 3.6'
            },
            {
                id: 'none',
                visible: true,
                name: 'PyPy'
            },
            {
                id: 'none',
                visible: true,
                name: 'NodeJS'
            },
            {
                id: 'none',
                visible: true,
                name: 'Shell'
            },
            {
                id: 'none',
                visible: true,
                name: 'Java'
            }
        ];

        ctrl.selectedEntryType = {
            id: 'none',
            visible: true,
            name: 'Git repository'
        };

        ctrl.selectedRuntime = {
            id: 'none',
            visible: true,
            name: 'NodeJS'
        };

        // Config for scrollbar on code-tab view
        ctrl.scrollConfig = {
            axis: 'xy',
            advanced: {
                autoScrollOnFocus: false,
                updateOnContentResize: true
            }
        };

        ctrl.sourceCode = atob(ctrl.version.spec.build.functionSourceCode);

        ctrl.selectEntryTypeValue = selectEntryTypeValue;
        ctrl.selectRuntimeValue = selectRuntimeValue;
        ctrl.onCloseDropdown = onCloseDropdown;
        ctrl.inputValueCallback = inputValueCallback;

        //
        // Public methods
        //

        /**
         * Sets new value to entity type
         * @param {Object} item
         */
        function selectEntryTypeValue(item) {
            ctrl.selectedEntryType = item;
        }

        /**
         * Sets new value to runtime
         * @param {Object} item
         */
        function selectRuntimeValue(item) {
            ctrl.selectedRuntime = item;
        }

        /**
         * Handles on drop-down close
         */
        function onCloseDropdown() {
            $timeout(function () {
                var element = angular.element('.tab-content-wrapper');
                var targetElement = $element.find('.default-dropdown-container');

                if (targetElement.length > 0 && ctrl.selectedEntryType.name !== 'Edit online') {
                    PreventDropdownCutOffService.resizeScrollBarContainer(element, '.default-dropdown-container');
                }
            }, 40);
        }

        /**
         * Update data callback
         * @param {string} newData
         * @param {string} field
         */
        function inputValueCallback(newData, field) {
            lodash.set(ctrl.version, field, newData);
        }
    }
}());

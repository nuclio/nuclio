(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclFunctionCollapsingRow', {
            bindings: {
                function: '<',
                project: '<',
                actionHandlerCallback: '&'
            },
            templateUrl: 'projects/project/functions/function-collapsing-row/function-collapsing-row.tpl.html',
            controller: NclFunctionCollapsingRowController
        });

    function NclFunctionCollapsingRowController(lodash, NuclioFunctionsDataService) {
        var ctrl = this;

        ctrl.actions = [];
        ctrl.isCollapsed = false;

        ctrl.$onInit = onInit;

        ctrl.isFunctionShowed = isFunctionShowed;
        ctrl.handleAction = handleAction;
        ctrl.onFireAction = onFireAction;

        //
        // Hook methods
        //

        /**
         * Initialization method
         */
        function onInit() {
            lodash.defaultsDeep(ctrl.function, {
                ui: {
                    delete: deleteFunction
                }
            });

            ctrl.actions = initActions();
        }

        //
        // Public methods
        //

        /**
         * According to given action name calls proper action handler
         * @param {string} actionType
         * @param {Array} checkedItems
         * @returns {Promise}
         */
        function handleAction(actionType, checkedItems) {
            ctrl.actionHandlerCallback({actionType: actionType, checkedItems: checkedItems});
        }

        /**
         * Determines whether the current layer is showed
         * @returns {boolean}
         */
        function isFunctionShowed() {
            return ctrl.function.ui.isShowed;
        }

        /**
         * According to given action name calls proper action handler
         * @param {string} actionType - a type of action
         */
        function onFireAction(actionType) {
            ctrl.actionHandlerCallback({actionType: actionType, checkedItems: [ctrl.function]});
        }

        //
        // Private methods
        //

        /**
         * Initializes actions
         * @returns {Object[]} - list of actions
         */
        function initActions() {
            return [
                {
                    label: 'Delete',
                    id: 'delete',
                    icon: 'igz-icon-trash',
                    active: true,
                    capability: 'nuclio.functions.delete',
                    confirm: {
                        message: 'Delete Function “' + ctrl.function.metadata.name + '”?',
                        description: 'Deleted function cannot be restored.',
                        yesLabel: 'Yes, Delete',
                        noLabel: 'Cancel',
                        type: 'nuclio_alert'
                    }
                }
            ];
        }

        /**
         * Deletes function from functions list
         * @returns {Promise}
         */
        function deleteFunction() {
            return NuclioFunctionsDataService.deleteFunction(ctrl.function.metadata);
        }
    }
}());

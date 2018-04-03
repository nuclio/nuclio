(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclVersionTrigger', {
            bindings: {
                version: '<'
            },
            templateUrl: 'projects/project/functions/version/version-trigger/version-trigger.tpl.html',
            controller: NclVersionTriggerController
        });

    function NclVersionTriggerController(lodash, DialogsService) {
        var ctrl = this;

        ctrl.isCreateModeActive = false;
        ctrl.triggers = [];

        ctrl.$onInit = onInit;
        ctrl.createTrigger = createTrigger;
        ctrl.editTriggerCallback = editTriggerCallback;
        ctrl.handleAction = handleAction;

        //
        // Hook methods
        //

        /**
         * Initialization method
         */
        function onInit() {
            lodash.defaultsDeep(ctrl.version, {
                spec: {
                    triggers: {}
                }
            });

            // get trigger list
            ctrl.triggers = [];
            lodash.forOwn(ctrl.version.spec.triggers, function (value, key) {
                value.id = key;
                value.name = key;
                ctrl.triggers.push(value);
            });
        }

        //
        // Public methods
        //

        /**
         * Toggle create trigger mode
         * @returns {Promise}
         */
        function createTrigger(event) {
            ctrl.triggers.push({
                id: '',
                name: '',
                kind: '',
                url: '',
                attributes: {},
                ui: {
                    editModeActive: true,
                    expanded: true
                }
            });
            event.stopPropagation();
        }

        /**
         * Edit trigger callback function
         * @returns {Promise}
         */
        function editTriggerCallback(item) {
            ctrl.handleAction('update', item);

            item.ui.editModeActive = false;
            item.ui.expanded = false;
        }

        /**
         * According to given action name calls proper action handler
         * @param {string} actionType - ex. `delete`
         * @param {Array} selectedItem - an object of selected trigger
         * @returns {Promise}
         */
        function handleAction(actionType, selectedItem) {
            if (actionType === 'delete') {
                lodash.remove(ctrl.triggers, ['id', selectedItem.id]);
                lodash.unset(ctrl.version, 'spec.triggers.' + selectedItem.id);
            } else if (actionType === 'edit') {
                lodash.find(ctrl.triggers, ['id', selectedItem.id]).ui.editModeActive = true;
            } else if (actionType === 'update') {
                if (!lodash.isEmpty(selectedItem.id)) {
                    lodash.unset(ctrl.version, 'spec.triggers.' + selectedItem.id);
                }
                var triggerItem = {
                    kind: selectedItem.kind,
                    url: selectedItem.url,
                    attributes: selectedItem.attributes
                };
                lodash.set(ctrl.version, 'spec.triggers.' + selectedItem.name, triggerItem);
            } else {
                DialogsService.alert('This functionality is not implemented yet.');
            }
        }
    }
}());

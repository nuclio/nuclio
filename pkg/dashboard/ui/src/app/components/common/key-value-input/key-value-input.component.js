(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclKeyValueInput', {
            bindings: {
                actionHandlerCallback: '&',
                changeDataCallback: '&',
                itemIndex: '<',
                rowData: '<',
                useType: '<'
            },
            templateUrl: 'common/key-value-input/key-value-input.tpl.html',
            controller: NclKeyValueInputController
        });

    function NclKeyValueInputController($document, $element, $scope, lodash) {
        var ctrl = this;

        ctrl.data = {};
        ctrl.editMode = false;
        ctrl.typesList = [];

        ctrl.$onInit = onInit;

        ctrl.getInputValue = getInputValue;
        ctrl.getType = getType;
        ctrl.inputValueCallback = inputValueCallback;
        ctrl.onFireAction = onFireAction;
        ctrl.onTypeChanged = onTypeChanged;

        //
        // Hook methods
        //

        /**
         * Initialization method
         */
        function onInit() {
            ctrl.data = lodash.cloneDeep(ctrl.rowData);

            ctrl.actions = initActions();
            ctrl.typesList = getTypesList();

            $document.on('click', saveChanges);
            $document.on('keypress', saveChanges);
        }

        //
        // Public methods
        //

        /**
         * Gets model for value input
         * @returns {string}
         */
        function getInputValue() {
            if (ctrl.useType) {
                var specificType = ctrl.getType() === 'value'     ? 'value' :
                                   ctrl.getType() === 'configmap' ? 'valueFrom.configMapKeyRef' : 'valueFrom.secretKeyRef';
                var value = lodash.get(ctrl.data, specificType);

                return specificType === 'value' ? value :
                    value.name + (!lodash.isNil(value.key) ? ':' + value.key : '');
            } else {
                return ctrl.data.value;
            }
        }

        /**
         * Gets selected type
         * @returns {string}
         */
        function getType() {
            return !ctrl.useType || lodash.isNil(ctrl.data.valueFrom) ? 'value'     :
                   lodash.isNil(ctrl.data.valueFrom.secretKeyRef)     ? 'configmap' : 'secret';
        }

        /**
         * Update data callback
         * @param {string} newData
         * @param {string} field
         */
        function inputValueCallback(newData, field) {
            if (lodash.includes(field, 'value') && ctrl.getType() !== 'value') {
                var keyValueData = newData.split(':');

                lodash.set(ctrl.data, getValueField(), {
                    name: keyValueData[0]
                });

                if (keyValueData.length > 1) {
                    lodash.assign(lodash.get(ctrl.data, getValueField()), {
                        key: keyValueData[1]
                    });
                }
            } else {
                ctrl.data[field] = newData;
            }
        }

        /**
         * According to given action name calls proper action handler
         * @param {string} actionType - a type of action
         */
        function onFireAction(actionType) {
            if (actionType === 'edit') {
                ctrl.editMode = true;

                $document.on('click', saveChanges);
                $document.on('keypress', saveChanges);
            } else {
                ctrl.actionHandlerCallback({actionType: actionType, index: ctrl.itemIndex});

                ctrl.editMode = false;
            }
        }

        /**
         * Callback method which handles field type changing
         * @param {Object} newType - type selected in dropdown
         * @param {boolean} isItemChanged - shows whether item was changed
         */
        function onTypeChanged(newType, isItemChanged) {
            if (isItemChanged) {
                if (newType.id === 'secret' || newType.id === 'configmap') {
                    var specificType = newType.id === 'secret' ? 'secretKeyRef' : 'configMapKeyRef';
                    var value = {
                        name: ''
                    };

                    ctrl.data = lodash.omit(ctrl.data, ['value', 'valueFrom']);
                    lodash.set(ctrl.data, 'valueFrom.' + specificType, value);
                } else {
                    ctrl.data = lodash.omit(ctrl.data, 'valueFrom');
                    lodash.set(ctrl.data, 'value', '');
                }
            }
        }

        //
        // Private method
        //

        /**
         * Gets types list
         * @returns {Array.<Object>}
         */
        function getTypesList() {
            return [
                {
                    id: 'value',
                    name: 'Value'
                },
                {
                    id: 'secret',
                    name: 'Secret'
                },
                {
                    id: 'configmap',
                    name: 'Configmap'
                }
            ];
        }

        /**
         * Gets field which should be setted from value input
         * @returns {string}
         */
        function getValueField() {
            return !ctrl.useType || ctrl.getType() === 'value' ? 'value' :
                ctrl.getType() === 'configmap'              ? 'valueFrom.configMapKeyRef' : 'valueFrom.secretKeyRef';
        }

        /**
         * Gets actions
         * @returns {Array.<Object>}
         */
        function initActions() {
            return [
                {
                    label: 'Edit',
                    id: 'edit',
                    icon: 'igz-icon-edit',
                    active: true,
                    capability: 'nuclio.functions.versions.edit'
                },
                {
                    label: 'Delete',
                    id: 'delete',
                    icon: 'igz-icon-trash',
                    active: true,
                    capability: 'nuclio.functions.versions.delete',
                    confirm: {
                        message: 'Are you sure you want to delete selected item?',
                        yesLabel: 'Yes, Delete',
                        noLabel: 'Cancel',
                        type: 'critical_alert'
                    }
                }
            ];
        }

        /**
         * Calls callback with new data
         * @param {Event} event
         */
        function saveChanges(event) {
            if ($element.find(event.target).length === 0 || event.which === 13) {
                $scope.$evalAsync(function () {
                    ctrl.editMode = false;

                    $document.off('click', saveChanges);
                    $document.off('keypress', saveChanges);

                    ctrl.changeDataCallback({newData: ctrl.data, index: ctrl.itemIndex});
                });
            }
        }
    }
}());

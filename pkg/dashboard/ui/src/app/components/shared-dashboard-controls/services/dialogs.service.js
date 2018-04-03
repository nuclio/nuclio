(function () {
    'use strict';

    angular.module('iguazio.app')
        .factory('DialogsService', DialogsService);

    function DialogsService($q, lodash, ngDialog, FormValidationService) {
        return {
            alert: alert,
            confirm: confirm,
            customConfirm: customConfirm,
            image: image,
            oopsAlert: oopsAlert,
            prompt: prompt,
            text: text
        };

        //
        // Public methods
        //

        /**
         * Show alert message
         *
         * @param {string|Array.<string>} [alertText] - alert content
         * @param {string} [buttonText=OK] - text displayed on Ok button
         * @returns {Promise} a promise that resolves on closing dialog
         */
        function alert(alertText, buttonText) {
            buttonText = lodash.defaultTo(buttonText, 'OK');

            if (angular.isArray(alertText)) {
                alertText = alertText.length === 1 ? lodash.first(alertText) :
                    '<ul class="error-list"><li class="error-list-item">' +
                        alertText.join('</li><li class="error-list-item">') + '</li></ul>';
            }

            return ngDialog.open({
                template: '<div class="notification-text title igz-scrollable-container" data-ng-scrollbars>' + alertText + '</div>' +
                '<div class="buttons">' +
                '<div class="igz-button-primary" data-ng-click="closeThisDialog() || $event.stopPropagation()">' + buttonText + '</div>' +
                '</div>',
                plain: true
            })
                .closePromise;
        }

        /**
         * Show confirmation dialog
         *
         * @param {string|Object} confirmText that will be shown in pop-up
         * @param {string} confirmButton Text displayed on Confirm button
         * @param {string} [cancelButton=Cancel] Text displayed on Cancel button
         * @param {string} type - type of popup dialog
         * @returns {Object}
         */
        function confirm(confirmText, confirmButton, cancelButton, type) {
            var confirmMessage = !lodash.isNil(type) && type === 'nuclio_alert' && lodash.isPlainObject(confirmText) ?
                confirmText.message : confirmText;

            if (!cancelButton) {
                cancelButton = 'Cancel';
            }

            var template = '<div class="close-button igz-icon-close" data-ng-click="closeThisDialog()"></div>' +
                '<div class="nuclio-alert-icon"></div><div class="notification-text title">' + confirmMessage + '</div>' +
                (!lodash.isNil(type) && type === 'nuclio_alert' && !lodash.isNil(confirmText.description) ?
                '<div class="notification-text description">' + confirmText.description + '</div>' : '') +
                '<div class="buttons">' +
                '<div class="igz-button-just-text" tabindex="0" data-ng-click="closeThisDialog(0)" data-ng-keydown="$event.keyCode === 13 && closeThisDialog(0)">' + cancelButton + '</div>' +
                '<div class="' +
                (!lodash.isNil(type) && (type === 'critical_alert' || type === 'nuclio_alert') ? 'igz-button-remove' : 'igz-button-primary') +
                '" tabindex="0" data-ng-click="confirm(1)" data-ng-keydown="$event.keyCode === 13 && confirm(1)">' + confirmButton + '</div>' +
                '</div>';

            return ngDialog.openConfirm({
                template: template,
                plain: true,
                trapFocus: false,
                className: !lodash.isNil(type) && type === 'nuclio_alert' ? 'ngdialog-theme-nuclio delete-entity-dialog-wrapper' : 'ngdialog-theme-iguazio'
            });
        }

        /**
         * Show confirmation dialog with custom number of buttons
         * @param {string} confirmText that will be shown in pop-up
         * @param {string} cancelButton Text displayed on Cancel button
         * @param {Array} actionButtons Array of action buttons
         * @returns {Object}
         */
        function customConfirm(confirmText, cancelButton, actionButtons) {
            var template = '<div class="notification-text title">' + confirmText + '</div>' +
                '<div class="buttons">' +
                '<div class="igz-button-just-text" tabindex="0" data-ng-click="closeThisDialog(-1)" data-ng-keydown="$event.keyCode === 13 && closeThisDialog(-1)">' + cancelButton + '</div>';
            lodash.each(actionButtons, function (button, index) {
                template += '<div class="igz-button-primary" tabindex="0" data-ng-click="confirm(' +
                    index + ')" data-ng-keydown="$event.keyCode === 13 && confirm(' + index + ')">' + button + '</div>';
            });
            template += '</div>';

            return ngDialog.openConfirm({
                template: template,
                plain: true,
                trapFocus: false
            });
        }

        /**
         * Show image
         *
         * @param {string} src that will be shown in pop-up
         * @param {string} [label] actual filename to be shown in title
         * @returns {Promise}
         */
        function image(src, label) {
            label = angular.isString(label) ? label : 'Image preview:';

            return ngDialog.open({
                template: '<div class="title text-ellipsis"' +
                'data-tooltip="' + label + '"' +
                'data-tooltip-popup-delay="400"' +
                'data-tooltip-append-to-body="true"' +
                'data-tooltip-placement="bottom-left">' + label + '</div>' +
                '<div class="close-button igz-icon-close" data-ng-click="closeThisDialog()"></div>' +
                '<div class="image-preview-container">' +
                '<img class="image-preview" src="' + src + '" alt="You have no permissions to read the file"/></div>',
                plain: true,
                className: 'ngdialog-theme-iguazio image-dialog'
            })
                .closePromise;
        }

        /**
         * Show oops alert message when server is unreachable
         * @param {string} alertText that will be shown in pop-up
         * @param {string} buttonText displayed on Ok button
         * @returns {Promise}
         */
        function oopsAlert(alertText, buttonText) {
            return ngDialog.open({
                template: '<div class="header"></div><div class="notification-text">' + alertText + '</div>' +
                '<div class="buttons">' +
                '<div class="refresh-button" data-ng-click="closeThisDialog()"><span class="igz-icon-refresh"></span>' + buttonText + '</div>' +
                '</div>',
                plain: true,
                className: 'ngdialog-theme-iguazio oops-dialog'
            })
                .closePromise;
        }

        /**
         * Show confirmation dialog with input field
         *
         * @param {string} promptText that will be shown in pop-up
         * @param {string} confirmButton Text displayed on Confirm button
         * @param {string} [cancelButton='Cancel'] Text displayed on Cancel button
         * @param {string} [defaultValue=''] Value that should be shown in text input after prompt is opened
         * @param {string} [placeholder=''] Text input placeholder
         * @param {Object} [validation] Validation pattern
         * @param {boolean} required Should input be required or not
         * @returns {Object}
         */
        function prompt(promptText, confirmButton, cancelButton, defaultValue, placeholder, validation, required) {
            cancelButton = cancelButton || 'Cancel';
            placeholder = placeholder || '';
            defaultValue = defaultValue || '';

            var data = {
                value: defaultValue,
                igzDialogPromptForm: {},
                checkInput: function () {
                    if (angular.isDefined(validation) || required) {
                        data.igzDialogPromptForm.$submitted = true;
                    }
                    return data.igzDialogPromptForm.$valid;
                },
                inputValueCallback: function (newData) {
                    data.value = newData;
                }
            };

            if (angular.isDefined(validation) || required) {
                lodash.assign(data, {
                    validation: validation,
                    inputName: 'promptName',
                    isShowFieldInvalidState: FormValidationService.isShowFieldInvalidState
                });
            }

            return ngDialog.open({
                template: '<div data-ng-form="ngDialogData.igzDialogPromptForm">' +
                    '<div class="close-button igz-icon-close" data-ng-click="closeThisDialog()"></div>' +
                    '<div class="notification-text title">' + promptText + '</div>' +
                    '<div class="main-content">' +
                        '<div class="field-group">' +
                            '<div class="field-input">' +
                                '<igz-validating-input-field data-field-type="input" ' +
                                                            'data-input-name="promptName" ' +
                                                            'data-input-value="ngDialogData.value" ' +
                                                            'data-form-object="ngDialogData.igzDialogPromptForm" ' +
                                                            'data-is-focused="true" ' +
                                                            (angular.isDefined(validation) ? 'data-validation-pattern="ngDialogData.validation" ' : '') +
                                                            (placeholder !== '' ? 'data-placeholder-text="' + placeholder + '" ' : '') +
                                                            (required ? 'data-validation-is-required="true" ' : '') +
                                                            'data-update-data-callback="ngDialogData.inputValueCallback(newData)"' +
                                                            '>' +
                                '</igz-validating-input-field>' +
                                (angular.isDefined(validation) ? '<div class="error-text" data-ng-show="ngDialogData.isShowFieldInvalidState(ngDialogData.igzDialogPromptForm, ngDialogData.inputName)">' +
                                'The input is Invalid, please try again.' +
                                '</div>' : '') +
                            '</div>' +
                        '</div>' +
                    '</div>' +
                '</div>' +
                '<div class="buttons">' +
                    '<div class="igz-button-just-text" data-ng-click="closeThisDialog()">' + cancelButton + '</div>' +
                    '<div class="igz-button-primary" data-ng-click="ngDialogData.checkInput() && closeThisDialog(ngDialogData.value)">' + confirmButton + '</div>' +
                '</div>',
                plain: true,
                data: data
            })
                .closePromise
                .then(function (dialog) { // if Cancel is clicked, reject the promise
                    return angular.isDefined(dialog.value) ? dialog.value : $q.reject('Cancelled');
                });
        }

        /**
         * Shows text
         *
         * @param {string} content that will be shown in pop-up
         * @param {Object} [node] actual node to be shown
         * @param {function} submitData function for submitting data
         * @returns {Promise}
         */
        function text(content, node, submitData) {
            var data = {
                closeButtonText: 'Close',
                submitButtonText: 'Save',
                submitData: submitData,
                label: angular.isString(node.label) ? node.label : 'Text preview:',
                node: node,
                content: content
            };

            return ngDialog.open({
                template: '<igz-text-edit data-label="{{ngDialogData.label}}" data-content="{{ngDialogData.content}}"' +
                'data-submit-button-text="{{ngDialogData.submitButtonText}}" data-submit-data="ngDialogData.submitData(newContent)"' +
                'data-close-button-text="{{ngDialogData.closeButtonText}}" data-close-dialog="closeThisDialog()" data-node="ngDialogData.node">' +
                '</igz-text-edit>',
                plain: true,
                data: data,
                className: 'ngdialog-theme-iguazio text-edit'
            })
                .closePromise;
        }
    }
}());

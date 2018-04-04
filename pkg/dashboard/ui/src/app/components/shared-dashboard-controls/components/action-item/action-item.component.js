(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('igzActionItem', {
            bindings: {
                action: '<',
                actions: '<?',
                template: '@'
            },
            templateUrl: 'shared-dashboard-controls/components/action-item/action-item.tpl.html',
            controller: IgzActionItem
        });

    function IgzActionItem($rootScope, $scope, $element, $document, lodash, ContainerBrowserService, DialogsService) {
        var ctrl = this;

        ctrl.$onInit = onInit();
        ctrl.$onDestroy = onDestroy();
        ctrl.isItemVisible = isItemVisible;
        ctrl.onClickAction = onClickAction;
        ctrl.onFilesDropped = ContainerBrowserService.onFilesDropped;

        //
        // Hook methods
        //

        /**
         * Initialization method
         */
        function onInit() {
            lodash.defaults(ctrl.action, {
                visible: true
            });
        }

        /**
         * Destructor method
         */
        function onDestroy() {
            if (angular.isDefined(ctrl.action.template)) {
                detachDocumentEvent();
            }
        }

        //
        // Public methods
        //

        /**
         * Handles mouse click on action item
         * @param {MouseEvent} event
         */
        function onClickAction(event) {
            if (ctrl.action.active) {
                if (!lodash.isNil(ctrl.action.popupText)) {
                    $rootScope.$broadcast('browse-action_change-loading-text', {textToDisplay: ctrl.action.popupText});
                }

                // shows confirmation dialog if action.confirm is true
                if (lodash.isNonEmpty(ctrl.action.confirm)) {
                    showConfirmDialog(event);
                } else {
                    ctrl.action.handler(ctrl.action, event);
                }

                // if action has sub-templates shows/hides it
                if (angular.isDefined(ctrl.action.template)) {
                    toggleTemplate();
                }

                // calls callback if defined
                if (angular.isFunction(ctrl.action.callback)) {
                    ctrl.action.callback(ctrl.action);
                }
            }
        }

        /**
         * Checks if the action item should be shown
         * @param {Object} action
         * @returns {boolean}
         */
        function isItemVisible(action) {
            return lodash.get(action, 'visible', true);
        }

        //
        // Private methods
        //

        /**
         * Attaches on click event handler to the document
         */
        function attachDocumentEvent() {
            $document.on('click', hideSubtemplate);
        }

        /**
         * Removes on click event handler attached to the document
         */
        function detachDocumentEvent() {
            $document.off('click', hideSubtemplate);
        }

        /**
         * Hides sub-template dropdown when user clicks outside it
         * @param {MouseEvent} event
         */
        function hideSubtemplate(event) {
            $scope.$apply(function () {
                if (event.target !== $element[0] && $element.find(event.target).length === 0) {
                    ctrl.action.subTemplateProps.isShown = false;
                    detachDocumentEvent();
                }
            });
        }

        /**
         * Shows confirm dialog
         * @param {MouseEvent} event
         */
        function showConfirmDialog(event) {
            var message = lodash.isNil(ctrl.action.confirm.description) ? ctrl.action.confirm.message : {
                message: ctrl.action.confirm.message,
                description: ctrl.action.confirm.description
            };

            DialogsService.confirm(message, ctrl.action.confirm.yesLabel, ctrl.action.confirm.noLabel, ctrl.action.confirm.type)
                .then(function () {
                    ctrl.action.handler(ctrl.action, event);
                });
        }

        /**
         * Shows/hides sub-template
         */
        function toggleTemplate() {
            ctrl.action.subTemplateProps.isShown = !ctrl.action.subTemplateProps.isShown;
            if (ctrl.action.subTemplateProps.isShown) {
                attachDocumentEvent();
            } else {
                detachDocumentEvent();
            }
        }
    }
}());

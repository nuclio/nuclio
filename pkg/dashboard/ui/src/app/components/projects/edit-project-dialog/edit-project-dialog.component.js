(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclEditProjectDialog', {
            bindings: {
                project: '<',
                confirm: '&',
                closeDialog: '&'
            },
            templateUrl: 'projects/edit-project-dialog/edit-project-dialog.tpl.html',
            controller: IgzEditProjectDialogController
        });

    function IgzEditProjectDialogController($scope, $rootScope, lodash, EventHelperService, FormValidationService,
                                            NuclioProjectsDataService) {
        var ctrl = this;

        ctrl.data = {};
        ctrl.isLoadingState = false;
        ctrl.nameTakenError = false;
        ctrl.serverError = '';

        ctrl.$onInit = onInit;

        ctrl.isShowFieldError = FormValidationService.isShowFieldError;
        ctrl.isShowFieldInvalidState = FormValidationService.isShowFieldInvalidState;

        ctrl.inputValueCallback = inputValueCallback;
        ctrl.isServerError = isServerError;
        ctrl.onClose = onClose;
        ctrl.saveProject = saveProject;

        //
        // Hook method
        //

        /**
         * Initialization method
         */
        function onInit() {
            ctrl.data = lodash.cloneDeep(ctrl.project);
        }

        //
        // Public methods
        //

        /**
         * Handle click on `Apply changes` button or press `Enter`
         * @param {Event} [event]
         */
        function saveProject(event) {
            if (angular.isUndefined(event) || event.keyCode === EventHelperService.ENTER) {
                $scope.editProjectForm.$submitted = true;

                if ($scope.editProjectForm.$valid) {
                    ctrl.isLoadingState = true;

                    // use data from dialog to create a new project
                    NuclioProjectsDataService.updateProject(ctrl.data)
                        .then(function () {
                            ctrl.confirm();
                        })
                        .catch(function (error) {
                            var status = lodash.get(error, 'data.errors[0].status');

                            ctrl.serverError =
                                status === 400                   ? 'Missing mandatory fields'                         :
                                status === 403                   ? 'You do not have permissions to update project'    :
                                status === 405                   ? 'Failed to create a project'                     :
                                lodash.inRange(status, 500, 599) ? 'Server error'                                     :
                                                                   'Unknown error occurred. Retry later';
                        })
                        .finally(function () {
                            ctrl.isLoadingState = false;
                        });
                }
            }
        }

        /**
         * Sets new data from input field for corresponding field of current project
         * @param {string} newData - new string value which should be set
         * @param {string} field - field name, ex. `name`, `description`
         */
        function inputValueCallback(newData, field) {
            lodash.set(ctrl.data, field, newData);
        }

        /**
         * Checks if server error is present or not
         * @returns {boolean}
         */
        function isServerError() {
            return ctrl.serverError !== '';
        }

        /**
         * Closes dialog
         * @param {Event} [event]
         */
        function onClose(event) {
            if ((angular.isUndefined(event) || event.keyCode === EventHelperService.ENTER) && !ctrl.isLoadingState) {
                ctrl.closeDialog();
            }
        }
    }
}());

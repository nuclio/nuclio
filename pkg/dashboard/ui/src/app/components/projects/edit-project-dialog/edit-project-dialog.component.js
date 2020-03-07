(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('nclEditProjectDialog', {
            bindings: {
                project: '<',
                closeDialog: '&',
            },
            templateUrl: 'projects/edit-project-dialog/edit-project-dialog.tpl.html',
            controller: IgzEditProjectDialogController
        });

    function IgzEditProjectDialogController($scope, $i18next, i18next, lodash, EventHelperService,
                                            FormValidationService, NuclioProjectsDataService,
                                            ValidatingPatternsService) {
        var ctrl = this;
        var lng = i18next.language;

        ctrl.data = {};
        ctrl.isLoadingState = false;
        ctrl.nameValidationRules = [];
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
            ctrl.nameValidationRules = ValidatingPatternsService.getValidationRules('k8s.dns1035Label');
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
                    var newProjectState = lodash.omit(ctrl.data, 'ui');
                    NuclioProjectsDataService.updateProject(newProjectState)
                        .then(function () {
                            ctrl.closeDialog({ project: newProjectState });
                        })
                        .catch(function (error) {
                            var defaultMsg = $i18next.t('functions:ERROR_MSG.UPDATE_PROJECT', {lng: lng});

                            ctrl.serverError = lodash.get(error, 'data.error', defaultMsg)
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

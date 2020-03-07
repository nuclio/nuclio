(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('nclNewProjectDialog', {
            bindings: {
                closeDialog: '&',
            },
            templateUrl: 'projects/new-project-dialog/new-project-dialog.tpl.html',
            controller: IgzNewProjectDialogController
        });

    function IgzNewProjectDialogController($scope, $i18next, i18next, lodash, moment, ConfigService, DialogsService,
                                           EventHelperService, FormValidationService, NuclioProjectsDataService,
                                           ValidatingPatternsService) {
        var ctrl = this;
        var lng = i18next.language;

        ctrl.data = {};
        ctrl.isLoadingState = false;
        ctrl.nameMaxLength = null;
        ctrl.nameValidationRules = [];
        ctrl.serverError = '';

        ctrl.$onInit = onInit;

        ctrl.isShowFieldError = FormValidationService.isShowFieldError;
        ctrl.isShowFieldInvalidState = FormValidationService.isShowFieldInvalidState;

        ctrl.createProject = createProject;
        ctrl.inputValueCallback = inputValueCallback;
        ctrl.isServerError = isServerError;
        ctrl.onClose = onClose;

        //
        // Hook method
        //

        /**
         * Initialization method
         */
        function onInit() {
            ctrl.data = getBlankData();
            ctrl.nameMaxLength = ValidatingPatternsService.getMaxLength('k8s.dns1035Label');
            ctrl.nameValidationRules = ValidatingPatternsService.getValidationRules('k8s.dns1035Label');
        }

        //
        // Public methods
        //

        /**
         * Handle click on `Create project` button or press `Enter`
         * @param {Event} [event]
         */
        function createProject(event) {
            if (angular.isUndefined(event) || event.keyCode === EventHelperService.ENTER) {
                $scope.newProjectForm.$submitted = true;

                if ($scope.newProjectForm.$valid) {
                    ctrl.isLoadingState = true;

                    if (ConfigService.isDemoMode()) {
                        lodash.defaultsDeep(ctrl.data, {
                            spec: {
                                created_by: 'admin',
                                created_date: moment().toISOString()
                            }
                        });
                    }

                    // use data from dialog to create a new project
                    NuclioProjectsDataService.createProject(ctrl.data)
                        .then(function () {
                            ctrl.closeDialog({ project: ctrl.data });
                        })
                        .catch(function (error) {
                            var defaultMsg = $i18next.t('common:ERROR_MSG.UNKNOWN_ERROR_RETRY_LATER', {lng: lng});

                            ctrl.serverError = lodash.get(error, 'data.error', defaultMsg)
                        })
                        .finally(function () {
                            ctrl.isLoadingState = false;
                        });
                }
            }
        }

        /**
         * Sets new data from input field for corresponding field of new project
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

        //
        // Private method
        //

        /**
         * Gets black data
         * @returns {Object} - black data
         */
        function getBlankData() {
            return {
                metadata: {
                    name: ''
                },
                spec: {
                    description: ''
                }
            };
        }
    }
}());

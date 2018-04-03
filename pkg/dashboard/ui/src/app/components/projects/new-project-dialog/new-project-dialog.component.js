(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclNewProjectDialog', {
            bindings: {
                closeDialog: '&'
            },
            templateUrl: 'projects/new-project-dialog/new-project-dialog.tpl.html',
            controller: IgzNewProjectDialogController
        });

    function IgzNewProjectDialogController($scope, lodash, moment, EventHelperService, FormValidationService,
                                           NuclioProjectsDataService) {
        var ctrl = this;

        ctrl.data = {};
        ctrl.isLoadingState = false;
        ctrl.nameTakenError = false;
        ctrl.nameValidationPattern = /^.{1,128}$/;
        ctrl.namespaceValidationPattern = /^.{1,128}$/;
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

                    // sets default `created_by` and `created_date` if they are not defined
                    lodash.defaultsDeep(ctrl.data, {
                        spec: {
                            created_by: 'nuclio',
                            created_date: moment().toISOString()
                        }
                    });

                    // use data from dialog to create a new project
                    NuclioProjectsDataService.createProject(ctrl.data)
                        .then(function () {
                            ctrl.data = getBlankData();

                            ctrl.closeDialog();
                        })
                        .catch(function (error) {
                            var status = lodash.get(error, 'data.errors[0].status');

                            ctrl.serverError =
                                status === 400                   ? 'Missing mandatory fields'                         :
                                status === 403                   ? 'You do not have permissions to create new projects' :
                                status === 405                   ? 'Failed to create a new project. '             +
                                                                   'The maximum number of projects is reached. '  +
                                                                   'An existing project should be deleted first ' +
                                                                   'before creating a new one.'                       :
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
                    namespace: ''
                },
                spec: {
                    displayName: '',
                    description: ''
                }
            };
        }
    }
}());

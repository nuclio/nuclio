(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclProjectsTableRow', {
            bindings: {
                project: '<',
                projectsList: '<',
                actionHandlerCallback: '&'
            },
            templateUrl: 'projects/projects-table-row/projects-table-row.tpl.html',
            controller: NclProjectsTableRowController
        });

    function NclProjectsTableRowController($scope, $state, lodash, moment, ngDialog, ActionCheckboxAllService, DialogsService,
                                           NuclioProjectsDataService) {
        var ctrl = this;

        ctrl.$onInit = onInit;

        ctrl.showDetails = showDetails;

        //
        // Hook method
        //

        /**
         * Initialization method
         */
        function onInit() {

            // initialize `deleteProject`, `editProjects` actions and assign them to `ui` property of current project
            // sets default `created_by` and `created_date` if they are not defined
            // initialize `checked` status to `false`
            lodash.defaultsDeep(ctrl.project, {
                spec: {
                    created_by: 'nuclio',
                    created_date: moment().toISOString()
                },
                ui: {
                    checked: false,
                    delete: deleteProject,
                    edit: editProject
                }
            });
        }

        //
        // Public method
        //

        /**
         * Handles mouse click on a project name
         * Navigates to Functions page
         * @param {MouseEvent} event
         * @param {string} [state=app.project.functions] - absolute state name or relative state path
         */
        function showDetails(event, state) {
            if (!angular.isString(state)) {
                state = 'app.project.functions';
            }

            event.preventDefault();
            event.stopPropagation();

            $state.go(state, {
                projectId: ctrl.project.metadata.name
            });
        }

        //
        // Private methods
        //

        /**
         * Deletes project from projects list
         */
        function deleteProject() {
            NuclioProjectsDataService.deleteProject(ctrl.project)
                .catch(function (error) {
                    var errorMessages = {
                        403: 'You do not have permissions to delete this project.',
                        default: 'Unknown error occurred while deleting the project.'
                    };

                    return DialogsService.alert(lodash.get(errorMessages, error.status, errorMessages.default));
                });
        }

        /**
         * Opens `Edit project` dialog
         */
        function editProject() {
            return ngDialog.openConfirm({
                template: '<ncl-edit-project-dialog data-project="$ctrl.project" data-confirm="confirm()"' +
                'data-close-dialog="closeThisDialog(newProject)"></ncl-edit-project-dialog>',
                plain: true,
                scope: $scope,
                className: 'ngdialog-theme-nuclio'
            })
                .then(function () {

                    // unchecks project before updating list
                    if (ctrl.project.ui.checked) {
                        ctrl.project.ui.checked = false;

                        ActionCheckboxAllService.changeCheckedItemsCount(-1);
                    }
                });
        }
    }
}());

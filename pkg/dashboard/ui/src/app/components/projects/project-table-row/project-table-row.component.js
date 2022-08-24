/*
Copyright 2017 The Nuclio Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('nclProjectTableRow', {
            bindings: {
                isSplashShowed: '&',
                project: '<',
                projectActionHandlerCallback: '&'
            },
            templateUrl: 'projects/project-table-row/project-table-row.tpl.html',
            transclude: true,
            controller: NclProjectTableRowController
        });

    function NclProjectTableRowController($q, $scope, $state, $i18next, i18next, lodash, moment, ngDialog,
                                          ActionCheckboxAllService, ConfigService, ExportService, ProjectsService,
                                          NuclioFunctionsDataService, NuclioProjectsDataService) {
        var ctrl = this;
        var lng = i18next.language;

        ctrl.projectActions = {};

        ctrl.$onInit = onInit;
        ctrl.$onDestroy = onDestroy;

        ctrl.onFireAction = onFireAction;
        ctrl.onSelectRow = onSelectRow;

        ctrl.projectsService = ProjectsService;
        ctrl.isDemoMode = ConfigService.isDemoMode;

        //
        // Hook methods
        //

        /**
         * Initialization method
         */
        function onInit() {

            // initialize `checked` status to `false`
            lodash.defaultsDeep(ctrl.project, {
                ui: {
                    checked: false
                }
            });

            // assign `deleteProject`, `editProject`, `exportProject` actions to `ui` property of current project
            lodash.assign(ctrl.project.ui, {
                'delete': deleteProject,
                'edit': editProject,
                'export': exportProject
            });

            if (ConfigService.isDemoMode()) {
                lodash.defaultsDeep(ctrl.project, {
                    spec: {
                        created_by: 'admin',
                        created_date: moment().toISOString()
                    }
                });
            }

            initProjectActions();
        }

        /**
         * Destructor method
         */
        function onDestroy() {
            if (lodash.get(ctrl.project, 'ui.checked')) {
                lodash.set(ctrl.project, 'ui.checked', false);

                ActionCheckboxAllService.changeCheckedItemsCount(-1);
            }
        }

        //
        // Public method
        //

        /**
         * According to given action name calls proper action handler
         * @param {string} actionType - a type of action
         */
        function onFireAction(actionType) {
            ctrl.projectActionHandlerCallback({ actionType: actionType, checkedItems: [ctrl.project] });
        }

        /**
         * Handles mouse click on a project name
         * Navigates to Functions page
         * @param {MouseEvent} event
         * @param {string} [state=app.project.functions] - absolute state name or relative state path
         */
        function onSelectRow(event, state) {
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
         * @returns {Promise}
         */
        function deleteProject() {
            var projectId = lodash.get(ctrl.project, 'metadata.name');

            return NuclioProjectsDataService.deleteProject(ctrl.project)
                .then(function () {
                    return projectId;
                })
                .catch(function (error) {
                    var errorText = lodash.get(error, 'status') === 412     &&
                    lodash.startsWith(error.data.error, 'Project contains') ?
                        $i18next.t('functions:ERROR_MSG.DELETE_NOT_EMPTY_PROJECT', { lng: lng }) :
                        lodash.get(error, 'data.error', $i18next.t('functions:ERROR_MSG.DELETE_PROJECT', { lng: lng }));

                    return $q.reject({ errorText: errorText, errorStatus: lodash.get(error, 'status'),
                        errorMessage: error.data.error });
                });
        }

        /**
         * Opens `Edit project` dialog
         */
        function editProject() {
            return ngDialog.open({
                template: '<ncl-edit-project-dialog ' +
                    'data-project="$ctrl.project"' +
                    'data-close-dialog="closeThisDialog(project)" ' +
                    '</ncl-edit-project-dialog>',
                plain: true,
                scope: $scope,
                className: 'ngdialog-theme-nuclio nuclio-project-edit-dialog'
            }).closePromise
                .then(function (data) {
                    if (!lodash.isNil(data.value)) {
                        return data.value;
                    }
                })
                .catch(function () {
                    return $q.reject($i18next.t('functions:ERROR_MSG.UPDATE_PROJECT', { lng: lng }));
                });
        }

        /**
         * Exports the project
         * @returns {Promise}
         */
        function exportProject() {
            ExportService.exportProject(ctrl.project, NuclioFunctionsDataService.getFunctions);
            return $q.when();
        }

        /**
         * Initializes project actions
         * @returns {Object[]} - list of project actions
         */
        function initProjectActions() {
            ctrl.projectActions = angular.copy(ProjectsService.initProjectActions());

            var deleteAction = lodash.find(ctrl.projectActions, { id: 'delete' });

            if (!lodash.isNil(deleteAction)) {
                deleteAction.confirm.message =
                    $i18next.t('functions:DELETE_PROJECT', { lng: lng }) + ' “' + ctrl.project.metadata.name + '“?'
            }
        }
    }
}());

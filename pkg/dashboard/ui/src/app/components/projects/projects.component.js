/* eslint max-statements: ["error", 100] */
/* eslint max-params: ["error", 25] */
(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('nclProjects', {
            templateUrl: 'projects/projects.tpl.html',
            controller: ProjectsController
        });

    function ProjectsController($element, $q, $rootScope, $scope, $state, $timeout, $transitions, $i18next, i18next,
                                lodash, ngDialog, ActionCheckboxAllService, CommonTableService, ConfigService,
                                DialogsService, ExportService, GeneralDataService, ImportService, NuclioFunctionsDataService,
                                NuclioProjectsDataService, ProjectsService) {
        var ctrl = this;
        var lng = i18next.language;

        ctrl.checkedItemsCount = 0;
        ctrl.dropdownActions = [
            {
                id: 'exportProjects',
                name: $i18next.t('functions:EXPORT_ALL_PROJECTS', { lng: lng })
            },
            {
                id: 'importProject',
                name: $i18next.t('functions:IMPORT_PROJECTS', { lng: lng })
            }
        ];
        ctrl.filtersCounter = 0;
        ctrl.isFiltersShowed = {
            value: false,
            changeValue: function (newVal) {
                this.value = newVal;
            }
        };
        ctrl.isReverseSorting = false;
        ctrl.isSplashShowed = {
            value: true
        };
        ctrl.projectActions = [];
        ctrl.projects = [];
        ctrl.searchKeys = [
            'metadata.name',
            'spec.displayName',
            'spec.description'
        ];
        ctrl.searchStates = {};
        ctrl.selectedProject = {};
        ctrl.sortOptions = [
            {
                label: $i18next.t('common:NAME', { lng: lng }),
                value: 'metadata.name',
                active: true,
                desc: false
            },
            {
                label: $i18next.t('common:DESCRIPTION', { lng: lng }),
                value: 'spec.description',
                active: false
            }
        ];
        ctrl.sortedColumnName = 'metadata.name';
        ctrl.sortedProjects = [];
        ctrl.versionActions = [];

        ctrl.$onInit = onInit;

        ctrl.handleProjectAction = handleProjectAction;
        ctrl.importProject = importProject;
        ctrl.isProjectsListEmpty = isProjectsListEmpty;
        ctrl.onApplyFilters = onApplyFilters;
        ctrl.onResetFilters = onResetFilters;
        ctrl.onSelectDropdownAction = onSelectDropdownAction;
        ctrl.onSortOptionsChange = onSortOptionsChange;
        ctrl.onUpdateFiltersCounter = onUpdateFiltersCounter;
        ctrl.openNewFunctionScreen = openNewFunctionScreen;
        ctrl.openNewProjectDialog = openNewProjectDialog;
        ctrl.refreshProjects = refreshProjects;
        ctrl.sortTableByColumn = sortTableByColumn;
        ctrl.toggleFilters = toggleFilters;

        ctrl.getColumnSortingClasses = CommonTableService.getColumnSortingClasses;
        ctrl.isDemoMode = ConfigService.isDemoMode;
        ctrl.projectsService = ProjectsService;

        //
        // Hook methods
        //

        /**
         * Initialization method
         */
        function onInit() {

            // initializes project actions array
            ctrl.projectActions = angular.copy(ProjectsService.initProjectActions());

            ctrl.isSplashShowed.value = true;

            updateProjects(true)
                .then(function () {
                    ctrl.isSplashShowed.value = false;

                    updatePanelActions();
                    sortTable();
                })
                .finally(function () {
                    $timeout(function () {
                        $rootScope.$broadcast('igzWatchWindowResize::resize');
                    });
                });

            $scope.$on('action-panel_fire-action', onFireAction);
            $scope.$on('action-checkbox-all_checked-items-count-change', updatePanelActions);
            $scope.$on('action-checkbox-all_check-all', updatePanelActions);

            $transitions.onStart({}, stateChangeStart);
        }

        //
        // Public methods
        //

        /**
         * According to given action name calls proper action handler
         * @param {string} actionType - e.g. `'delete'`, `'edit'`
         * @param {Array} projects - an array of checked projects
         * @returns {Promise}
         */
        function handleProjectAction(actionType, projects) {
            var messagesArray = [];
            var notEmptyMessagesArray = [];
            var promises = lodash.map(projects, function (project) {
                var projectName = getName(project);
                return lodash.result(project, 'ui.' + actionType)
                    .then(function (result) {
                        if (actionType === 'edit') {

                            // update the row in view
                            lodash.merge(project, result);
                        } else if (actionType === 'delete') {

                            // un-check project
                            if (project.ui.checked) {
                                project.ui.checked = false;

                                ActionCheckboxAllService.changeCheckedItemsCount(-1);
                            }

                            // remove from list
                            lodash.pull(ctrl.projects, project);
                            sortTable();
                        }
                    })
                    .catch(function (errorMessage) {
                        var messageData = { name: projectName, message: errorMessage.errorText,
                            status: errorMessage.errorStatus }
                        if (errorMessage.errorStatus === 412 && lodash.startsWith(errorMessage.errorMessage,
                                                                                  'Project contains')) {
                            notEmptyMessagesArray.push(messageData);
                        } else {
                            messagesArray.push(messageData);
                        }
                    });
            });

            return $q.all(promises)
                .then(function () {
                    if (lodash.isNonEmpty(messagesArray) || lodash.isNonEmpty(notEmptyMessagesArray)) {
                        var messages = formatMessage(messagesArray);
                        var notEmptyMessages = formatMessage(notEmptyMessagesArray);
                        var promise = lodash.isEmpty(messages) ? $q.when() : DialogsService.alert(messages);

                        if (lodash.isNonEmpty(notEmptyMessagesArray)) {
                            return promise.then(function () {
                                confirmDelete(notEmptyMessages).then(function () {
                                    var notEmptyProjects = lodash.chain(ctrl.projects)
                                        .filter(function (project) {
                                            return lodash.includes(lodash.map(notEmptyMessagesArray, 'name'),
                                                                   project.metadata.name)
                                        })
                                        .map(function (project) {
                                            return lodash.set(project, 'ui.forceDelete', true)
                                        })
                                        .value()

                                    return handleProjectAction('delete', notEmptyProjects)
                                })
                            })
                        }
                    }
                });
        }

        /**
         * Imports project and updates the projects list
         * @param {File} file
         */
        function importProject(file) {
            ImportService.importFile(file)
                .then(updateProjects);
        }

        /**
         * Checks if projects list is empty
         * @returns {boolean}
         */
        function isProjectsListEmpty() {
            return lodash.isEmpty(ctrl.projects);
        }

        /**
         * Updates projects list depends on filters value
         */
        function onApplyFilters() {
            $rootScope.$broadcast('search-input_refresh-search');
        }

        /**
         * Handles on reset filters event
         */
        function onResetFilters() {
            $rootScope.$broadcast('search-input_reset');

            ctrl.filtersCounter = 0;
        }

        /**
         * Called when dropdown action is selected
         * @param {Object} item - selected action
         */
        function onSelectDropdownAction(item) {
            if (item.id === 'exportProjects') {
                ExportService.exportProjects(ctrl.projects, NuclioFunctionsDataService.getFunctions);
            } else if (item.id === 'importProject') {
                $element.find('.project-import-input').click();
            }
        }

        /**
         * Sorts the table by column name depends on selected value in sort dropdown.
         * @param {Object} option - Selected option.
         */
        function onSortOptionsChange(option) {
            ctrl.isReverseSorting = option.desc;
            ctrl.sortedColumnName = option.value;

            sortTable();
        }

        /**
         * Handles on update filters counter
         * @param {string} searchQuery
         */
        function onUpdateFiltersCounter(searchQuery) {
            ctrl.filtersCounter = lodash.isEmpty(searchQuery) ? 0 : 1;
        }

        /**
         * Navigates to New Function screen
         */
        function openNewFunctionScreen() {
            $state.go('app.create-function', {
                navigatedFrom: 'projects'
            });
        }

        /**
         * Creates and opens new project dialog
         */
        function openNewProjectDialog() {
            ngDialog.open({
                template: '<ncl-new-project-dialog data-close-dialog="closeThisDialog(project)" ' +
                    '</ncl-new-project-dialog>',
                plain: true,
                scope: $scope,
                className: 'ngdialog-theme-nuclio nuclio-project-create-dialog'
            })
                .closePromise
                .then(function (data) {
                    if (!lodash.isNil(data.value)) {
                        updateProjects();
                    }
                });
        }

        /**
         * Refreshes projects list
         */
        function refreshProjects() {
            updateProjects();
        }

        /**
         * Sorts the table by column.
         * @param {string} columnName - The name of the column to sort by.
         */
        function sortTableByColumn(columnName) {
            // set the sorting order (ascending if selected a different column, or toggle if selected the same column)
            ctrl.isReverseSorting = columnName === ctrl.sortedColumnName ? !ctrl.isReverseSorting : false;

            // save the name of the column to sort by
            ctrl.sortedColumnName = columnName;

            sortTable();
        }

        /**
         * Shows/hides filters panel
         */
        function toggleFilters() {
            ctrl.isFiltersShowed.value = !ctrl.isFiltersShowed.value;
        }

        //
        // Private methods
        //

        /**
         * Returns pop-up dialog
         * @param {string} confirmMessage
         * @returns {string}
         */
        function confirmDelete(confirmMessage) {
            var template = '<div class="close-button igz-icon-close" data-ng-click="closeThisDialog()"></div>' +
                '<div class="nuclio-alert-icon"></div>' + '<div class="notification-text title">' + confirmMessage +
                '</div>' + '<div class="buttons" >'  +
                '<button class="igz-button-just-text" tabindex="0"  data-ng-click="closeThisDialog(0)" >' +
                $i18next.t('common:CANCEL', { lng: i18next.language }) +
                '<button class="igz-button-primary" tabindex="0" data-ng-click="confirm(1)">' +
                $i18next.t('common:DELETE', { lng: i18next.language }) +
                '</button>' + '</button>' + '</div>';

            return ngDialog.openConfirm({
                template: template,
                plain: true,
                name: 'confirm',
                className: 'ngdialog-theme-nuclio'
            });
        }

        /**
         * Formats error messages to be displayed
         * @param {array} errorMessages
         * @returns {string}
         */
        function formatMessage(errorMessages) {
            var formattedErrorMessage = '';
            if (errorMessages.length === 1) {
                formattedErrorMessage = errorMessages[0].message;
            } else {
                lodash.each(errorMessages, function (errorMessage) {
                    formattedErrorMessage = formattedErrorMessage +
                        errorMessage.name + ': ' + errorMessage.message + '<br />';
                })
            }

            return formattedErrorMessage;
        }

        /**
         * Returns correct project name
         * @param {Object} project
         * @returns {string}
         */
        function getName(project) {
            return project.metadata.name;
        }

        /**
         * Gets a list of all projects
         * @returns {Promise}
         */
        function getProjects() {
            return NuclioProjectsDataService.getProjects()
                .then(function (projectsFromResponse) {
                    ctrl.projects = lodash.map(projectsFromResponse, function (projectFromResponse) {
                        var foundProject =
                            lodash.find(ctrl.projects, ['metadata.name', projectFromResponse.metadata.name]);
                        var ui = lodash.get(foundProject, 'ui');
                        projectFromResponse.ui = lodash.defaultTo(ui, projectFromResponse.ui);

                        return projectFromResponse;
                    });

                    if (lodash.isEmpty(ctrl.projects)) {
                        $state.go('app.nuclio-welcome');
                    }

                    sortTable();
                })
                .catch(function (error) {
                    if (!GeneralDataService.isDisconnectionError(error.status)) {
                        var defaultMsg = $i18next.t('functions:ERROR_MSG.GET_PROJECTS', { lng: lng });

                        DialogsService.alert(lodash.get(error, 'data.error', defaultMsg));
                    }
                });
        }

        /**
         * Handler on action-panel broadcast
         * @param {Event} event - $broadcast-ed event
         * @param {Object} data - $broadcast-ed data
         */
        function onFireAction(event, data) {
            ctrl.handleProjectAction(data.action, lodash.filter(ctrl.projects, 'ui.checked'));
        }

        /**
         * Sorts table according to the current sort-by column and sorting order (ascending/descending).
         */
        function sortTable() {
            ctrl.sortedProjects =
                lodash.orderBy(ctrl.projects, [ctrl.sortedColumnName], ctrl.isReverseSorting ? ['desc'] : ['asc']);
        }

        /**
         * Opens a splash screen on start change state
         */
        function stateChangeStart() {
            ctrl.isSplashShowed.value = true;
        }

        /**
         * Updates actions of action panel according to selected nodes
         * @param {Object} event - triggering event
         * @param {Object} data - passed data
         */
        function updatePanelActions(event, data) {
            var checkedRows = lodash.filter(ctrl.projects, 'ui.checked');
            var checkedRowsCount = lodash.get(data, 'checkedCount') || checkedRows.length;

            if (checkedRowsCount > 0) {

                // sets visibility status of `edit action`
                // visible if only one project is checked
                var editAction = lodash.find(ctrl.projectActions, { id: 'edit' });

                if (!lodash.isNil(editAction)) {
                    editAction.visible = checkedRowsCount === 1;
                }

                // sets confirm message for `delete action` depending on count of checked rows
                var deleteAction = lodash.find(ctrl.projectActions, { id: 'delete' });

                if (!lodash.isNil(deleteAction)) {
                    deleteAction.confirm.message = checkedRowsCount === 1 ?
                        $i18next.t('functions:DELETE_PROJECT', { lng: lng }) + ' “' + getName(checkedRows[0]) + '”?' :
                        $i18next.t('functions:DELETE_PROJECTS_CONFIRM', { lng: lng });
                }
            }
        }

        /**
         * Updates current projects
         * @param {boolean} [hideSplashScreen=false]
         * @returns {Promise}
         */
        function updateProjects(hideSplashScreen) {
            if (!hideSplashScreen) {
                ctrl.isSplashShowed.value = true;
            }

            return getProjects()
                .finally(function () {
                    if (!hideSplashScreen) {
                        ctrl.isSplashShowed.value = false;
                    }
                });
        }
    }
}());

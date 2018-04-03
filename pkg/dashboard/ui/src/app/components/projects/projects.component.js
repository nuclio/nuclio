(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclProjects', {
            templateUrl: 'projects/projects.tpl.html',
            controller: NclProjectsController
        });

    function NclProjectsController($scope, $q, $state, lodash, ngDialog, ActionCheckboxAllService, CommonTableService,
                                   NuclioProjectsDataService, ValidatingPatternsService) {
        var ctrl = this;

        ctrl.actions = [];
        ctrl.activeFilters = [];
        ctrl.checkedItemsCount = 0;
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
        ctrl.filter = {
            name: ''
        };
        ctrl.nameValidationPattern = ValidatingPatternsService.name;
        ctrl.projects = [];
        ctrl.readOnly = true;
        ctrl.searchStates = {};
        ctrl.searchKeys = [
            'attr.name'
        ];
        ctrl.selectedProject = {};
        ctrl.sortedColumnName = 'name';

        ctrl.$onInit = onInit;
        ctrl.$onDestroy = onDestroy;

        ctrl.isColumnSorted = CommonTableService.isColumnSorted;

        ctrl.clearFilterInput = clearFilterInput;
        ctrl.getActiveFilters = getActiveFilters;
        ctrl.updateProjects = updateProjects;
        ctrl.handleAction = handleAction;
        ctrl.isProjectsListEmpty = isProjectsListEmpty;
        ctrl.onApplyFilters = onApplyFilters;
        ctrl.onResetFilters = onResetFilters;
        ctrl.openNewProjectDialog = openNewProjectDialog;
        ctrl.refreshProjects = refreshProjects;
        ctrl.sortTableByColumn = sortTableByColumn;
        ctrl.toggleFilters = toggleFilters;

        //
        // Hook methods
        //

        /**
         * Initialization method
         */
        function onInit() {

            // initializes projects actions array
            ctrl.actions = initActions();

            // TODO pagination

            updateProjects();

            $scope.$on('action-panel_fire-action', onFireAction);
            $scope.$on('action-checkbox-all_check-all', updatePanelActions);
            $scope.$on('action-checkbox_item-checked', updatePanelActions);
        }

        /**
         * Destructor method
         */
        function onDestroy() {
            ngDialog.close();
        }

        //
        // Public methods
        //

        /**
         * Clears filter's field by name
         * @param {string} filterName - name of the filter
         */
        function clearFilterInput(filterName) {

            // TODO
        }

        /**
         * Gets active filters object
         * @returns {Object} - an object with active filters
         */
        function getActiveFilters() {

            // TODO
        }

        /**
         * Updates current projects
         */
        function updateProjects() {
            ctrl.isSplashShowed.value = true;

            NuclioProjectsDataService.getProjects().then(function (response) {
                ctrl.projects = lodash.values(response);

                if (lodash.isEmpty(ctrl.projects)) {
                    $state.go('app.nuclio-welcome');
                } else {
                    ctrl.isSplashShowed.value = false;
                }
            });
        }

        /**
         * According to given action name calls proper action handler
         * @param {string} actionType - ex. `delete`
         * @param {Array} checkedItems - an array of checked projects
         * @returns {Promise}
         */
        function handleAction(actionType, checkedItems) {
            var promises = [];

            lodash.forEach(checkedItems, function (checkedItem) {
                var actionHandler = checkedItem.ui[actionType];

                if (lodash.isFunction(actionHandler)) {
                    promises.push(actionHandler());
                }
            });

            return $q.all(promises).then(function () {
                if (actionType === 'delete') {
                    lodash.forEach(checkedItems, function (checkedItem) {
                        lodash.remove(ctrl.projects, ['metadata.name', checkedItem.metadata.name]);

                        // unchecks deleted project
                        if (checkedItem.ui.checked) {
                            ActionCheckboxAllService.changeCheckedItemsCount(-1);
                        }
                    });
                } else {
                    ActionCheckboxAllService.setCheckedItemsCount(0);

                    ctrl.refreshProjects();
                }
            });
        }

        /**
         * Checks if functions list is empty
         * @returns {boolean}
         */
        function isProjectsListEmpty() {
            return lodash.isEmpty(ctrl.projects);
        }

        /**
         * Updates projects list depends on filters value
         */
        function onApplyFilters() {

            // TODO
        }

        /**
         * Handles on reset filters event
         */
        function onResetFilters() {

            // TODO
        }

        /**
         * Creates and opens new project dialog
         */
        function openNewProjectDialog() {
            ngDialog.open({
                template: '<ncl-new-project-dialog data-close-dialog="closeThisDialog()"></ncl-new-project-dialog>',
                plain: true,
                scope: $scope,
                className: 'ngdialog-theme-nuclio'
            })
                .closePromise
                .then(updateProjects);
        }

        /**
         * Refreshes users list
         */
        function refreshProjects() {
            updateProjects();
        }

        /**
         * Sorts the table by column name
         * @param {string} columnName - name of column
         * @param {boolean} isJustSorting - if it is needed just to sort data without changing reverse
         */
        function sortTableByColumn(columnName, isJustSorting) {

            // TODO
        }

        /**
         * Show/hide filters panel
         */
        function toggleFilters() {
            ctrl.isFiltersShowed.value = !ctrl.isFiltersShowed.value;
        }

        //
        // Private methods
        //

        /**
         * Handler on action-panel broadcast
         * @param {Event} event - $broadcast-ed event
         * @param {Object} data - $broadcast-ed data
         * @param {string} data.action - a name of action
         */
        function onFireAction(event, data) {
            ctrl.handleAction(data.action, lodash.filter(ctrl.projects, 'ui.checked'));
        }

        /**
         * Actions for Action panel
         * @returns {Object[]}
         */
        function initActions() {
            return [
                {
                    label: 'Delete',
                    id: 'delete',
                    icon: 'igz-icon-trash',
                    active: true,
                    capability: 'nuclio.projects.delete',
                    confirm: {
                        message: 'Delete selected projects?',
                        yesLabel: 'Yes, Delete',
                        noLabel: 'Cancel',
                        type: 'nuclio_alert'
                    }
                },
                {
                    label: 'Edit',
                    id: 'edit',
                    icon: 'igz-icon-properties',
                    active: true,
                    capability: 'nuclio.projects.edit'
                }
            ];
        }

        /**
         * Updates actions of action panel according to selected nodes
         */
        function updatePanelActions() {
            var checkedRows = lodash.filter(ctrl.projects, 'ui.checked');
            if (checkedRows.length > 0) {

                // sets visibility status of `edit action`
                // visible if only one project is checked
                var editAction = lodash.find(ctrl.actions, {'id': 'edit'});
                if (!lodash.isNil(editAction)) {
                    editAction.visible = checkedRows.length === 1;
                }

                // sets confirm message for `delete action` depending on count of checked rows
                var deleteAction = lodash.find(ctrl.actions, {'id': 'delete'});
                if (!lodash.isNil(deleteAction)) {
                    var message = checkedRows.length === 1 ?
                        'Delete project “' + checkedRows[0].spec.displayName + '”?' : 'Delete selected projects?';

                    deleteAction.confirm = {
                        message: message,
                        description: 'Deleted project cannot be restored.',
                        yesLabel: 'Yes, Delete',
                        noLabel: 'Cancel',
                        type: 'nuclio_alert'
                    };
                }
            }
        }
    }
}());

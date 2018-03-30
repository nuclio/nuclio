(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclFunctions', {
            templateUrl: 'projects/project/functions/functions.tpl.html',
            controller: FunctionsController
        });

    function FunctionsController($q, $scope, $state, $stateParams, $timeout, lodash, HeaderService, NuclioHeaderService,
                                 NuclioProjectsDataService, NuclioFunctionsDataService) {
        var ctrl = this;

        ctrl.actions = [];
        ctrl.functions = [];
        ctrl.isFiltersShowed = {
            value: false,
            changeValue: function (newVal) {
                this.value = newVal;
            }
        };
        ctrl.isSplashShowed = {
            value: true
        };
        ctrl.project = {};
        ctrl.sortOptions = [
            {
                label: 'Name',
                value: 'name',
                active: true,
                desc: false
            },
            {
                label: 'Description',
                value: 'description',
                active: false,
                desc: false
            },
            {
                label: 'Status',
                value: 'status',
                active: false,
                desc: false
            }
        ];
        ctrl.title = {};

        ctrl.$onInit = onInit;

        ctrl.getVersions = getVersions;
        ctrl.handleAction = handleAction;
        ctrl.isFunctionsListEmpty = isFunctionsListEmpty;
        ctrl.onApplyFilters = onApplyFilters;
        ctrl.onResetFilters = onResetFilters;
        ctrl.openNewFunctionScreen = openNewFunctionScreen;
        ctrl.refreshFunctions = refreshFunctions;
        ctrl.toggleFilters = toggleFilters;

        //
        // Hook methods
        //

        /**
         * Initialization method
         */
        function onInit() {
            if (angular.isDefined($stateParams.projectId)) {
                ctrl.isSplashShowed.value = true;

                NuclioProjectsDataService.getProject($stateParams.projectId)
                    .then(function (project) {
                        ctrl.project = project;

                        ctrl.title = {
                            project: ctrl.project.spec.displayName
                        };

                        ctrl.refreshFunctions();

                        NuclioHeaderService.updateMainHeader('Projects', ctrl.title, $state.current.name);
                    });
            } else {
                ctrl.refreshFunctions();
            }

            ctrl.actions = NuclioFunctionsDataService.initVersionActions();

            $scope.$on('$stateChangeStart', stateChangeStart);
            $scope.$on('action-panel_fire-action', onFireAction);
            $scope.$on('action-checkbox_item-checked', updatePanelActions);
            $scope.$on('action-checkbox-all_check-all', function () {
                $timeout(updatePanelActions);
            });

            updatePanelActions();
        }

        //
        // Public methods
        //

        /**
         * Applies the current set of filters so further requests to retrieve a page of results will use these filters
         */
        function onApplyFilters() {
            // TODO
        }

        /**
         * Gets list of function versions
         * @returns {string[]}
         */
        function getVersions() {
            return lodash.chain(ctrl.functions)
                .map(function (functionItem) {

                    // TODO
                    return functionItem.version === -1 ? [] : functionItem.versions;
                })
                .flatten()
                .value();
        }

        /**
         * Checks if functions list is empty
         * @returns {boolean}
         */
        function isFunctionsListEmpty() {
            return lodash.isEmpty(ctrl.functions);
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
                        lodash.remove(ctrl.functions, ['metadata.name', checkedItem.metadata.name]);
                    });
                } else {
                    ctrl.refreshFunctions();
                }
            });
        }

        /**
         * Applies the current set of filters so further requests to retrieve a page of results will use these filters
         */
        function onResetFilters() {
            // TODO
        }

        /**
         * Navigates to new function screen
         */
        function openNewFunctionScreen() {
            ctrl.title.function = 'Create function';

            NuclioHeaderService.updateMainHeader('Projects', ctrl.title, $state.current.name);

            $state.go('app.project.create-function');
        }

        /**
         * Refreshes function list
         */
        function refreshFunctions() {
            ctrl.isSplashShowed.value = true;

            NuclioFunctionsDataService.getFunctions(ctrl.project.metadata.namespace).then(function (result) {
                ctrl.functions = lodash.toArray(result.data);

                // TODO mocked versions data
                lodash.forEach(ctrl.functions, function (functionItem) {
                    lodash.set(functionItem, 'versions', [{
                        name: '$LATEST',
                        invocation: '30',
                        last_modified: '2018-02-05T17:07:48.509Z'
                    }]);
                    lodash.set(functionItem, 'spec.version', 1);
                });

                ctrl.isSplashShowed.value = false;
            });
        }

        /**
         * Opens a splash screen on start change state
         */
        function stateChangeStart() {
            ctrl.isSplashShowed.value = true;
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
         * Handler on action-panel broadcast
         * @param {Event} event - $broadcast-ed event
         * @param {Object} data - $broadcast-ed data
         * @param {string} data.action - a name of action
         */
        function onFireAction(event, data) {
            var checkedRows = lodash.chain(ctrl.functions)
                .map(function (functionItem) {
                    return lodash.filter(functionItem.versions, 'ui.checked');
                })
                .flatten()
                .value();

            ctrl.handleAction(data.action, checkedRows);
        }

        /**
         * Updates actions of action panel according to selected versions
         */
        function updatePanelActions() {
            var checkedRows = lodash.chain(ctrl.functions)
                .map(function (functionItem) {
                    return lodash.filter(functionItem.versions, 'ui.checked');
                })
                .flatten()
                .value();

            if (checkedRows.length > 0) {

                // sets visibility status of `edit action`
                // visible if only one version is checked
                var editAction = lodash.find(ctrl.actions, {'id': 'edit'});
                if (!lodash.isNil(editAction)) {
                    editAction.visible = checkedRows.length === 1;
                }

                // sets confirm message for `delete action` depending on count of checked rows
                var deleteAction = lodash.find(ctrl.actions, {'id': 'delete'});
                if (!lodash.isNil(deleteAction)) {
                    var message = checkedRows.length === 1 ?
                        'Delete version “' + checkedRows[0].name + '”?' : 'Are you sure you want to delete selected version?';

                    deleteAction.confirm = {
                        message: message,
                        yesLabel: 'Yes, Delete',
                        noLabel: 'Cancel',
                        type: 'nuclio_alert'
                    };
                }
            }
        }
    }
}());

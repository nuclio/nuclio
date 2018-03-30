(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('igzSearchInput', {
            bindings: {
                dataSet: '<',
                searchKeys: '<',
                searchStates: '<',
                searchCallback: '&?',
                isSearchHierarchically: '@?',
                placeholder: '@',
                type: '@?',
                ruleType: '@?',
                searchType: '@?'
            },
            templateUrl: 'shared-dashboard-controls/components/search-input/search-input.tpl.html',
            controller: IgzSearchInputController
        });

    function IgzSearchInputController($scope, $timeout, lodash, SearchHelperService) {
        var ctrl = this;

        ctrl.isSearchHierarchically = (String(ctrl.isSearchHierarchically) === 'true');
        ctrl.searchQuery = '';

        ctrl.$onInit = onInit;
        ctrl.onPressEnter = onPressEnter;
        ctrl.clearInputField = clearInputField;

        //
        // Hook method
        //

        /**
         * Initialization method
         */
        function onInit() {
            ctrl.searchStates.searchNotFound = false;
            ctrl.searchStates.searchInProgress = false;
            if (angular.isUndefined(ctrl.searchType)) {
                ctrl.searchType = 'infoPage';
            }
            $scope.$watch('$ctrl.searchQuery', onChangeSearchQuery);
            $scope.$on('search-input_refresh-search', onDataChanged);
            $scope.$on('search-input_reset', resetSearch);
        }

        //
        // Public methods
        //

        /**
         * Initializes search on press enter
         * @param {Event} e
         */
        function onPressEnter(e) {
            if (e.keyCode === 13) {
                makeSearch();
            }
        }

        /**
         * Clear search input field
         */
        function clearInputField() {
            ctrl.searchQuery = '';
        }

        //
        // Private methods
        //

        /**
         * Calls service method for search
         */
        function makeSearch() {
            if (angular.isFunction(ctrl.searchCallback)) {

                // call custom search method
                ctrl.searchCallback(lodash.pick(ctrl, ['searchQuery', 'dataSet', 'searchKeys', 'isSearchHierarchically', 'ruleType', 'searchStates']));
            }

            if (angular.isUndefined(ctrl.type)) {

                // default search functionality
                SearchHelperService.makeSearch(ctrl.searchQuery, ctrl.dataSet, ctrl.searchKeys, ctrl.isSearchHierarchically,
                    ctrl.ruleType, ctrl.searchStates);
            }
        }

        /**
         * Tracks input changing and initializes search
         */
        function onChangeSearchQuery(newValue, oldValue) {
            if (angular.isDefined(newValue) && newValue !== oldValue) {
                makeSearch();
            }
        }

        /**
         * Initializes search when all html has been rendered
         */
        function onDataChanged() {
            $timeout(makeSearch);
        }

        /**
         * Resets search query and initializes search
         */
        function resetSearch() {
            ctrl.searchQuery = '';
            $timeout(makeSearch);
        }
    }
}());

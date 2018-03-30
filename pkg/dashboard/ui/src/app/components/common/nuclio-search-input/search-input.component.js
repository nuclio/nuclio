(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclSearchInput', {
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
            templateUrl: 'common/nuclio-search-input/search-input.tpl.html',
            controller: SearchInputController
        });

    function SearchInputController($scope, $timeout) {
        var ctrl = this;

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
            $scope.$watch('$ctrl.searchQuery', onChangeSearchQuery);
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
            // TODO
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

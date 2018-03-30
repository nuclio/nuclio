(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclProjectsSortDropdown', {
            bindings: {
                sortOptions: '<',
                updateDataCallback: '&?'
            },
            templateUrl: 'projects/projects-sort-dropdown/projects-sort-dropdown.tpl.html',
            controller: NclProjectsSortDropdownController
        });

    function NclProjectsSortDropdownController() {
        var ctrl = this;
    }
}());

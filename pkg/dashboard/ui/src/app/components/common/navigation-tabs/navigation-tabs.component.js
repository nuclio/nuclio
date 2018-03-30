(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclNavigationTabs', {
            bindings: {
                tabItems: '<'
            },
            templateUrl: 'common/navigation-tabs/navigation-tabs.tpl.html'
        });
}());

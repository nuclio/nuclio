(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('igzActionItemRefresh', {
            bindings: {
                refresh: '&'
            },
            templateUrl: 'shared-dashboard-controls/components/action-item/action-item-refresh/action-item-refresh.tpl.html'
        });
}());

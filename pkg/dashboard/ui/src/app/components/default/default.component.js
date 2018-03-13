(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('igzDefault', {
            templateUrl: 'default/default.tpl.html',
            controller: DefaultController
        });

    function DefaultController() {
        var ctrl = this;
    }
}());

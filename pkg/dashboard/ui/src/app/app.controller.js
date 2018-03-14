(function () {
    'use strict';

    angular.module('iguazio.app')
        .controller('AppController', AppController);

    function AppController() {
        var ctrl = this;

        ctrl.pageTitle = 'Empty project | iguazio';
    }
}());

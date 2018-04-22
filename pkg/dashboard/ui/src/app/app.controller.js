(function () {
    'use strict';

    angular.module('nuclio.app')
        .controller('AppController', AppController);

    function AppController() {
        var ctrl = this;

        ctrl.pageTitle = 'Dashboard | nuclio';
    }
}());

(function () {
    'use strict';

    angular.module('nuclio.app')
        .controller('AppController', AppController);

    function AppController($transitions, $i18next, i18next) {
        var ctrl = this;
        var lng = i18next.language;

        activate();

        function activate() {
            $transitions.onSuccess({}, function (event) {
                var toState = event.$to();
                if (angular.isDefined(toState.data.pageTitle)) {
                    ctrl.pageTitle = $i18next.t(toState.data.pageTitle, {lng: lng}) + ' | Nuclio';
                }
            });
        }
    }
}());

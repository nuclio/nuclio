(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclFunction', {
            bindings: {},
            templateUrl: 'projects/project/functions/function/ncl-function.tpl.html',
            controller: NclFunctionController
        });

    function NclFunctionController($state, $timeout, ConfigService, HeaderService) {
        var ctrl = this;
    }
}());

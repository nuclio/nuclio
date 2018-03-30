(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclProject', {
            bindings: {},
            templateUrl: 'projects/project/ncl-project.tpl.html',
            controller: NclProjectController
        });

    function NclProjectController($state, $timeout, ConfigService, HeaderService) {
        var ctrl = this;
    }
}());

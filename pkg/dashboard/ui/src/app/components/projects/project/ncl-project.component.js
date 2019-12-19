(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('nclProject', {
            bindings: {},
            templateUrl: 'projects/project/ncl-project.tpl.html',
            controller: NclProjectController
        });

    function NclProjectController() {
        var ctrl = this;
    }
}());

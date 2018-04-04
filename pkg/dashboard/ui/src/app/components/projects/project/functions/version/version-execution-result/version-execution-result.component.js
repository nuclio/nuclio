(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclVersionExecutionResult', {
            bindings: {
                testResult: '<',
                toggleMethod: '&'
            },
            templateUrl: 'projects/project/functions/version/version-execution-result/version-execution-result.tpl.html'
        });
}());

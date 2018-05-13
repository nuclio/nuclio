(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('nclVersionInfoDialog', {
            bindings: {
                version: '<',
                closeDialog: '&'
            },
            templateUrl: 'header/version-info-dialog/version-info-dialog.tpl.html'
        });
}());

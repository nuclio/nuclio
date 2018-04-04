(function () {
    'use strict';

    angular.module('iguazio.app')
        .component('nclProjectsWelcomePage', {
            templateUrl: 'projects/projects-welcome-page/projects-welcome-page.tpl.html',
            controller: NclProjectsWelcomePageController
        });

    function NclProjectsWelcomePageController($scope, $state, ngDialog) {
        var ctrl = this;

        ctrl.$onDestroy = onDestroy;

        ctrl.openNewProjectDialog = openNewProjectDialog;

        //
        // Hook method
        //

        /**
         * Destructor method
         */
        function onDestroy() {
            ngDialog.close();
        }

        //
        // Public method
        //

        /**
         * Handle click on `Create new project` button
         * @param {Object} event
         */
        function openNewProjectDialog(event) {
            ngDialog.open({
                template: '<ncl-new-project-dialog data-close-dialog="closeThisDialog(newProject)"></ncl-new-project-dialog>',
                plain: true,
                scope: $scope,
                className: 'ngdialog-theme-nuclio new-project-dialog-wrapper'
            }).closePromise
                .then(function () {
                    $state.go('app.projects');
                });
        }
    }
}());

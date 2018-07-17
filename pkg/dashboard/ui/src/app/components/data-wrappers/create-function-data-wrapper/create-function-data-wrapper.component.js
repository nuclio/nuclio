(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('createFunctionDataWrapper', {
            templateUrl: 'data-wrappers/create-function-data-wrapper/create-function-data-wrapper.tpl.html',
            controller: CreateFunctionDataWrapperController
        });

    function CreateFunctionDataWrapperController(lodash, NuclioProjectsDataService, NuclioFunctionsDataService) {
        var ctrl = this;

        ctrl.templates = {};

        ctrl.getProject = getProject;
        ctrl.getTemplates = getTemplates;

        //
        // Public methods
        //

        /**
         * Gets a single project
         * @param {string} id - project ID
         * @returns {Promise}
         */
        function getProject(id) {
            return NuclioProjectsDataService.getProject(id);
        }

        /**
         * Gets list of function's templates
         */
        function getTemplates() {
            return NuclioFunctionsDataService.getTemplates()
                .then(function (templates) {
                    lodash.forIn(templates.data, function (value) {
                        var title = value.metadata.name.split(':')[0] + ' (' + value.spec.runtime + ')';

                        ctrl.templates[title] = value;
                    });

                    return ctrl.templates;
                })
        }
    }
}());

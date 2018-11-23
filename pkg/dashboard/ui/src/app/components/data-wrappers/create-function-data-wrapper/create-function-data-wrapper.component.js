(function () {
    'use strict';

    angular.module('nuclio.app')
        .component('createFunctionDataWrapper', {
            templateUrl: 'data-wrappers/create-function-data-wrapper/create-function-data-wrapper.tpl.html',
            controller: CreateFunctionDataWrapperController
        });

    function CreateFunctionDataWrapperController(lodash, YAML, NuclioProjectsDataService, NuclioFunctionsDataService) {
        var ctrl = this;

        ctrl.templates = {};

        ctrl.createProject = createProject;
        ctrl.getProject = getProject;
        ctrl.getProjects = getProjects;
        ctrl.getTemplates = getTemplates;
        ctrl.renderTemplate = renderTemplate;

        //
        // Public methods
        //

        /**
         * Create a single project
         * @param {Object} project
         * @returns {Promise}
         */
        function createProject(project) {
            return NuclioProjectsDataService.createProject(project);
        }

        /**
         * Gets a single project
         * @param {string} id - project ID
         * @returns {Promise}
         */
        function getProject(id) {
            return NuclioProjectsDataService.getProject(id);
        }

        /**
         * Gets a list of all projects
         * @returns {Promise}
         */
        function getProjects() {
            return NuclioProjectsDataService.getProjects();
        }

        /**
         * Gets list of function's templates
         */
        function getTemplates() {
            return NuclioFunctionsDataService.getTemplates()
                .then(function (templates) {
                    lodash.forIn(templates, function (value) {
                        if (!lodash.has(value, 'rendered') && lodash.has(value, 'template')) {
                            var template = YAML.parse(value.template.replace(/{{\s.+\s}}/g, '"$&"'));

                            lodash.set(value, 'rendered.spec', template.spec);
                        }

                        if (!lodash.has(value, 'rendered.metadata.name')) {
                            lodash.set(value, 'rendered.metadata.name', lodash.get(value, 'metadata.name', 'default'));
                        }

                        var title = value.rendered.metadata.name.split(':')[0] + ' (' + value.rendered.spec.runtime + ')';

                        ctrl.templates[title] = value;
                    });

                    return ctrl.templates;
                })
        }

        /**
         * Render template data
         * @param {string} template
         * @returns {Promise}
         */
        function renderTemplate(template) {
            return NuclioFunctionsDataService.renderTemplate(template);
        }
    }
}());

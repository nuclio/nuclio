(function () {
    'use strict';

    angular.module('nuclio.app')
        .factory('ImportService', ImportService);

    function ImportService($q, NuclioFunctionsDataService, NuclioProjectsDataService, lodash, YAML) {
        return {
            importProject: importProject
        };

        /**
         * Imports new project and deploy all functions of this project
         * @param {Object} file
         * @returns {Promise}
         */
        function importProject(file) {
            var reader = new FileReader();
            var importProjectsDeferred = $q.defer();

            reader.onload = function () {
                var projects = YAML.parse(reader.result).projects;

                lodash.forEach(projects, function (project) {
                    var projectName = lodash.get(project, 'metadata.name');
                    var projectData = {
                        metadata: {},
                        spec: {
                            displayName: projectName
                        }
                    };

                    NuclioProjectsDataService.createProject(projectData).then(function () {
                        NuclioProjectsDataService.getProjects().then(function (response) {
                            var functions = lodash.get(project, 'spec.functions');
                            var currentProject = lodash.find(response, ['spec.displayName', projectName]);
                            var projectID = lodash.get(currentProject, 'metadata.name');

                            lodash.forEach(functions, function (func) {
                                NuclioFunctionsDataService.updateFunction(func, projectID);
                            });

                            importProjectsDeferred.resolve();
                        });
                    });
                });

            };

            reader.readAsText(file);
            return importProjectsDeferred.promise;
        }
    }
}());

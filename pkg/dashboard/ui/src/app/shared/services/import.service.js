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
            var importProjectDeferred = $q.defer();

            reader.onload = function () {
                var newProject = YAML.parse(reader.result);
                var projectName = lodash.get(newProject, 'project.metadata.name');

                var projectData = {
                    spec: {
                        displayName: projectName
                    }
                };

                NuclioProjectsDataService.createProject(projectData).then(function () {
                    NuclioProjectsDataService.getProjects().then(function (projects) {
                        var functions = lodash.get(newProject, 'project.spec.functions');
                        var currentProject = lodash.find(projects, ['spec.displayName', projectName]);
                        var projectID = lodash.get(currentProject, 'metadata.name');

                        lodash.forEach(functions, function (func) {
                            NuclioFunctionsDataService.updateFunction(func, projectID);
                        });

                        importProjectDeferred.resolve();
                    });
                });
            };

            reader.readAsText(file);
            return importProjectDeferred.promise;
        }
    }
}());

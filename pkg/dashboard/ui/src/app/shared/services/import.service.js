(function () {
    'use strict';

    angular.module('nuclio.app')
        .factory('ImportService', ImportService);

    function ImportService($q, $i18next, i18next, DialogsService, NuclioFunctionsDataService,
                           NuclioProjectsDataService, lodash, YAML) {
        return {
            importFile: importFile
        };

        //
        // Public methods
        //

        /**
         * Imports YAML file and imports one or more projects
         * @param {Object} file
         * @returns {Promise}
         */
        function importFile(file) {
            var reader = new FileReader();
            var importDeferred = $q.defer();

            reader.onload = function () {
                var importedData = YAML.parse(reader.result);

                if (lodash.has(importedData, 'project')) {
                    importProject(importedData.project, importDeferred);
                } else if (lodash.has(importedData, 'projects')) {
                    lodash.forEach(importedData.projects, function (project) {
                        importProject(project, importDeferred);
                    });
                }
            };

            reader.readAsText(file);
            return importDeferred.promise;
        }

        //
        // Private methods
        //

        /**
         * Imports new project and deploy all functions of this project
         * @param {Object} project
         * @param {Object} promise
         */
        function importProject(project, promise) {
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
                        NuclioFunctionsDataService.createFunction(func, projectID);
                    });

                    promise.resolve();
                });
            }).catch(function () {
                DialogsService.alert($i18next.t('common:ERROR_MSG.IMPORT_PROJECT', {lng: i18next.language}));
            });
        }
    }
}());

(function () {
    'use strict';

    angular.module('nuclio.app')
        .factory('ImportService', ImportService);

    function ImportService($q, $i18next, i18next, lodash, YAML, DialogsService, NuclioFunctionsDataService,
                           NuclioProjectsDataService) {
        var lng = i18next.language;

        return {
            importFile: importFile
        };

        //
        // Public methods
        //

        /**
         * Imports YAML file and imports one or more projects and their functions
         * @param {Blob} file
         * @returns {Promise}
         */
        function importFile(file) {
            return $q(function (resolve, reject) {
                var reader = new FileReader();

                reader.onload = function () {
                    try {
                        var importedData = YAML.parse(reader.result);

                        if (lodash.has(importedData, 'project')) {
                            importProject(importedData.project, resolve, reject);
                        } else if (lodash.has(importedData, 'projects')) {
                            lodash.forEach(importedData.projects, function (project) {
                                importProject(project, resolve, reject);
                            });
                        } else {
                            throw new Error('invalid yaml');
                        }
                    } catch (error) {
                        DialogsService.alert($i18next.t('common:ERROR_MSG.IMPORT_YAML_FILE', {lng: lng}))
                            .then(function () {
                                reject(error)
                            });
                    }
                };

                reader.readAsText(file);
            });
        }

        //
        // Private methods
        //

        /**
         * Imports new project and deploy all functions of this project
         * @param {Object} project
         * @param {function} [onSuccess] - a callback function to call on success (no arguments passed)
         * @param {function} [onFailure] - a callback function to call on failure (the error is passed as 1st argument)
         */
        function importProject(project, onSuccess, onFailure) {
            var projectData = lodash.omit(project, 'spec.functions');

            NuclioProjectsDataService.createProject(projectData)
                .catch(function (error) {

                    // swallow "409 Conflict" errors
                    // if a project with the same name already exist - merge its functions
                    // for any other kind of error - rethrow error
                    if (error.status !== 409) {
                        throw error;
                    }
                })
                .then(function () {
                    return NuclioProjectsDataService.getProjects();
                })
                .then(function () {
                    var projectName = lodash.get(project, 'metadata.name');
                    var functions = lodash.get(project, 'spec.functions');

                    lodash.forEach(functions, function (func) {
                        NuclioFunctionsDataService.createFunction(func, projectName);
                    });

                    if (lodash.isFunction(onSuccess)) {
                        onSuccess()
                    }
                })
                .catch(function (error) {
                    DialogsService.alert($i18next.t('common:ERROR_MSG.IMPORT_PROJECT', {lng: lng}))
                        .then(function () {
                            if (lodash.isFunction(onFailure)) {
                                onFailure(error)
                            }
                        });
                });
        }
    }
}());

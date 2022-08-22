/*
Copyright 2017 The Nuclio Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
(function () {
    'use strict';

    angular.module('nuclio.app')
        .factory('ImportService', ImportService);

    function ImportService($q, $i18next, i18next, lodash,ngDialog, YAML, DialogsService, NuclioFunctionsDataService,
                           NuclioProjectsDataService) {
        var lng = i18next.language;

        var conflictProjectsData = {};
        var displayAllOptions = false;
        var importProjectPromisesList = [];

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
                            displayAllOptions = false;
                            importProjectPromisesList.push(importProject(importedData.project, reject));
                        } else if (lodash.has(importedData, 'projects')) {
                            displayAllOptions = true;
                            lodash.forEach(importedData.projects, function (project) {
                                importProjectPromisesList.push(importProject(project, reject));
                            });
                        } else {
                            throw new Error('invalid yaml');
                        }
                    } catch (error) {
                        DialogsService.alert($i18next.t('common:ERROR_MSG.IMPORT_YAML_FILE', {lng: lng}))
                            .then(function () {
                                reject(error);
                            });
                    }

                    $q.all(importProjectPromisesList)
                        .then(function () {
                            resolve();
                            checkConflictFunctionsList();
                        });
                };

                reader.readAsText(file);
            });
        }

        //
        // Private methods
        //

        /**
         * Checks if conflict functions list not empty
         */

        function checkConflictFunctionsList() {
            if (!lodash.isEmpty(conflictProjectsData)) {
                resolveConflict();
            }
        }

        /**
         * Imports new project and deploy all functions of this project
         * @param {Object} project
         * @param {function} [onFailure] - a callback function to call on failure (the error is passed as 1st argument)
         * @returns {Promise}
         */
        function importProject(project, onFailure) {
            var projectData = lodash.omit(project, 'spec.functions');
            var importProcess = true;

            return NuclioProjectsDataService.createProject(projectData, importProcess)
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

                    var checkedFunctionList = lodash.map(functions, function (func) {
                        return NuclioFunctionsDataService.createFunction(func, projectName, importProcess)
                            .catch(function (error) {
                                if (error.status === 409) {
                                    if (lodash.has(conflictProjectsData, projectName)) {
                                        conflictProjectsData[projectName].push(func);
                                        return conflictProjectsData[projectName];
                                    } else {
                                        return lodash.set(conflictProjectsData, [projectName], [func]);
                                    }
                                }
                            });
                    });

                    return $q.all(checkedFunctionList);
                })
                .catch(function (error) {
                    DialogsService.alert($i18next.t('common:ERROR_MSG.IMPORT_PROJECT', {lng: lng}))
                        .then(function () {
                            if (lodash.isFunction(onFailure)) {
                                onFailure(error);
                            }
                        });
                });
        }

        /**
         * Opens confirm dialog
         * @returns {Object} closeOptions
         */
        function openConfirmDialog(functionName, projectName) {
            var data = {
                displayAllOptions: displayAllOptions,
                dialogTitle: $i18next.t('common:OVERRIDE_FUNCTION_CONFIRM', {lng: lng, functionName: functionName, projectName: projectName})
            };

            return ngDialog.open({
                template: '<igz-import-project-dialog ' +
                    'data-display-all-options="ngDialogData.displayAllOptions" ' +
                    'data-close-dialog="closeThisDialog({action, option})" ' +
                    'data-dialog-title="ngDialogData.dialogTitle">' +
                    '</igz-import-project-dialog>',
                plain: true,
                data: data,
                className: 'ngdialog-theme-iguazio image-dialog'
            })
                .closePromise
                .then(function (closeOptions) {
                    return closeOptions;
                });
        }

        /**
         * Handles action and option for resolve conflict function
         */
        function resolveConflict() {
            var currentProject = lodash.keys(conflictProjectsData)[0];
            var currentFunction = lodash.get(conflictProjectsData, [currentProject] + '[0]');

            openConfirmDialog(currentFunction.metadata.name, currentProject)
                .then(function (data) {
                    if (data.value.option === 'allProjects') {
                        lodash.forEach(conflictProjectsData, function (functionsList, projectName) {
                            resolveConflictFunctionsInProject(functionsList, projectName, data.value.action, data.value.option);
                        });
                    } else {
                        var functions = data.value.option === 'singleProject' ? conflictProjectsData[currentProject] : [currentFunction];
                        resolveConflictFunctionsInProject(functions, currentProject, data.value.action, data.value.option);
                    }
                });
        }

        /**
         * Resolves conflict functions in a project
         * @param {Array.<Object>} functions
         * @param {string} projectID - project name
         * @param {string} action
         * @param {string} option
         */
        function resolveConflictFunctionsInProject(functions, projectID, action, option) {
            lodash.forEach(functions, function (func) {
                if (action === 'replace') {
                    NuclioFunctionsDataService.updateFunction(func, projectID);
                }
            });

            if (option === 'singleFunction') {
                conflictProjectsData[projectID].shift();
            }

            if (lodash.isEmpty(conflictProjectsData[projectID]) || option === 'singleProject') {
                lodash.unset(conflictProjectsData, [projectID])
            }

            if (option !== 'allProjects') {
                checkConflictFunctionsList();
            }
        }
    }
}());

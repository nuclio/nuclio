(function () {
    'use strict';

    angular.module('nuclio.app')
        .config(routes);

    function routes($stateProvider, $urlRouterProvider, lodash) {
        $urlRouterProvider.deferIntercept();

        $stateProvider
            .state('app', {
                abstract: true,
                url: '/',
                templateUrl: 'views/app/main.tpl.html',
                resolve: {
                    namespaceData: [
                        'NuclioNamespacesDataService',
                        function (NuclioNamespacesDataService) {
                            return NuclioNamespacesDataService.initNamespaceData();
                        }
                    ]
                }
            })
            .state('app.monaco', {
                url: 'monaco',
                views: {
                    main: {
                        template: '<ncl-monaco></ncl-monaco>'
                    }
                },
                data: {
                    pageTitle: 'common:MONACO'
                }
            })
            .state('app.nuclio-welcome', {
                url: 'welcome',
                views: {
                    main: {
                        template: '<ncl-projects-welcome-page></ncl-projects-welcome-page>'
                    }
                },
                data: {
                    pageTitle: 'common:WELCOME',
                    mainHeaderTitle: 'common:WELCOME'
                }
            })
            .state('app.projects', {
                url: 'projects',
                views: {
                    main: {
                        template: '<ncl-projects></ncl-projects>'
                    }
                },
                data: {
                    pageTitle: 'common:PROJECTS',
                    mainHeaderTitle: 'common:PROJECTS'
                },
                params: {
                    namespace: null
                }
            })
            .state('app.create-function', {
                url: 'projects/create-function',
                views: {
                    main: {
                        template: '<create-function-data-wrapper></create-function-data-wrapper>'
                    }
                },
                data: {
                    pageTitle: 'common:CREATE_FUNCTION',
                    mainHeaderTitle: 'common:PROJECTS',
                    capability: 'projects'
                },
                params: {
                    navigatedFrom: ''
                }
            })
            .state('app.project', {
                url: 'projects/:projectId',
                redirectTo: 'app.project.functions',
                views: {
                    main: {
                        template: '<ncl-project></ncl-project>'
                    }
                },
                params: {
                    createCancelled: false
                },
                data: {
                    pageTitle: 'common:FUNCTIONS',
                    mainHeaderTitle: 'common:FUNCTIONS'
                },
                resolve: {
                    project: [
                        'DialogsService', 'NuclioProjectsDataService', '$i18next', '$state', '$stateParams', 'i18next',
                        function (DialogsService, NuclioProjectsDataService, $i18next, $state, $stateParams, i18next) {
                            return NuclioProjectsDataService.getProject($stateParams.projectId)
                                .catch(function (error) {
                                    var defaultMsg =
                                        $i18next.t('functions:ERROR_MSG.GET_PROJECT', { lng: i18next.language });

                                    return DialogsService.alert(lodash.get(error, 'data.error', defaultMsg))
                                        .then(function () {
                                            $state.go('app.projects');
                                        });
                                });
                        }
                    ]
                }
            })
            .state('app.project.functions', {
                url: '/functions',
                views: {
                    project: {
                        template: '<functions-data-wrapper data-project="$resolve.project"></functions-data-wrapper>'
                    }
                },
                data: {
                    pageTitle: 'common:FUNCTIONS',
                    mainHeaderTitle: 'common:FUNCTIONS'
                }
            })
            .state('app.project.api-gateways', {
                url: '/api-gateways',
                views: {
                    project: {
                        template: '<api-gateways-data-wrapper data-project="$resolve.project"> ' +
                            '</api-gateways-data-wrapper>'
                    }
                },
                data: {
                    pageTitle: 'functions:API_GATEWAYS',
                    mainHeaderTitle: 'functions:API_GATEWAYS',
                },
                resolve: {
                    kubePlatform: ['$state', '$timeout', 'FunctionsService',
                        function ($state, $timeout, FunctionsService) {
                            return $timeout(function () {
                                if (!FunctionsService.isKubePlatform()) {
                                    $state.go('app.projects');
                                }
                            });
                        }]
                }
            })
            .state('app.project.create-function', {
                url: '/create-function',
                views: {
                    project: {
                        template: '<create-function-data-wrapper></create-function-data-wrapper>'
                    }
                },
                data: {
                    pageTitle: 'common:CREATE_FUNCTION'
                }
            })
            .state('app.project.function', {
                abstract: true,
                url: '/functions/:functionId',
                views: {
                    project: {
                        template: '<ncl-function></ncl-function>'
                    }
                },
                params: {
                    isNewFunction: false,
                    functionData: {}
                },
                data: {
                    pageTitle: 'common:FUNCTIONS',
                    mainHeaderTitle: 'common:FUNCTIONS'
                },
                resolve: {
                    function: [
                        'DialogsService', 'FunctionsService', 'NuclioFunctionsDataService', 'project', '$i18next',
                        '$state', '$stateParams', 'i18next',
                        function (DialogsService, FunctionsService, NuclioFunctionsDataService, project, $i18next,
                            $state, $stateParams, i18next) {
                            if ($stateParams.isNewFunction) {
                                return angular.copy($stateParams.functionData);
                            }

                            var functionMetadata = {
                                name: $stateParams.functionId,
                                namespace: project.metadata.namespace,
                                projectName: project.metadata.name
                            };

                            return NuclioFunctionsDataService.getFunction(functionMetadata, true)
                                .catch(function (error) {
                                    var defaultMsg =
                                        $i18next.t('functions:ERROR_MSG.GET_FUNCTION', { lng: i18next.language });
                                    return DialogsService.alert(lodash.get(error, 'data.error', defaultMsg))
                                        .then(function () {
                                            $state.go('app.project.functions', { projectId: $stateParams.projectId });
                                        });
                                });
                        }
                    ]
                }
            })
            .state('app.project.function.edit', {
                abstract: true,
                url: '',
                views: {
                    'function': {
                        template: '<version-data-wrapper data-project="$resolve.project" ' +
                            'data-version="$resolve.function"></version-data-wrapper>'
                    }
                },
                params: {
                    functionData: {}
                },
                data: {
                    pageTitle: 'common:EDIT_VERSION',
                    mainHeaderTitle: 'common:EDIT_VERSION'
                }
            })
            .state('app.project.function.edit.code', {
                url: '/code',
                views: {
                    version: {
                        template: '<ncl-version-code data-version="$ctrl.version"' +
                            'data-is-function-deploying="$ctrl.isFunctionDeploying()"></ncl-version-code>'
                    }
                },
                params: {
                    functionData: {}
                },
                data: {
                    pageTitle: 'common:CODE'
                }
            })
            .state('app.project.function.edit.configuration', {
                url: '/configuration',
                views: {
                    version: {
                        template: '<ncl-version-configuration data-version="$ctrl.version"' +
                            'data-is-function-deploying="$ctrl.isFunctionDeploying()"></ncl-version-configuration>'
                    }
                },
                params: {
                    functionData: {}
                },
                data: {
                    pageTitle: 'common:CONFIGURATION'
                }
            })
            .state('app.project.function.edit.triggers', {
                url: '/triggers',
                views: {
                    version: {
                        template: '<ncl-version-triggers data-version="$ctrl.version"' +
                            'data-is-function-deploying="$ctrl.isFunctionDeploying()"></ncl-version-triggers>'
                    }
                },
                params: {
                    functionData: {}
                },
                data: {
                    pageTitle: 'common:TRIGGERS'
                }
            })
            .state('app.project.function.edit.monitoring', {
                url: '/monitoring',
                views: {
                    version: {
                        template: '<ncl-version-monitoring data-version="$ctrl.version"></ncl-version-monitoring>'
                    }
                },
                params: {
                    functionData: {}
                },
                data: {
                    pageTitle: 'common:MONITORING'
                }
            });

        $urlRouterProvider
            .when('/projects/:id/functions/:functionId', '/projects/:id/functions/:functionId/code')
            .when('/projects/:id', '/projects/:id/functions')
            .when('/projects/', '/projects')

            .otherwise(function ($injector) {
                $injector.get('$state').go('app.projects');
            });
    }
}());

(function () {
    'use strict';

    angular.module('nuclio.app')
        .config(routes);

    function routes($stateProvider, $urlRouterProvider) {
        $urlRouterProvider.deferIntercept();

        $stateProvider
            .state('app', {
                abstract: true,
                url: '/',
                templateUrl: 'views/app/main.tpl.html'
            })
            .state('app.monaco', {
                url: 'monaco',
                views: {
                    main: {
                        template: '<ncl-monaco></ncl-monaco>'
                    }
                },
                data: {
                    pageTitle: 'Monaco'
                }
            })
            .state('app.nuclio-welcome', {
                url: 'welcome',
                views: {
                    main: {
                        template: '<projects-welcome-page-data-wrapper></projects-welcome-page-data-wrapper>'
                    }
                },
                data: {
                    pageTitle: 'Welcome',
                    mainHeaderTitle: 'Welcome'
                }
            })
            .state('app.projects', {
                url: 'projects',
                views: {
                    main: {
                        template: '<projects-data-wrapper></projects-data-wrapper>'
                    }
                },
                data: {
                    pageTitle: 'Projects',
                    mainHeaderTitle: 'Projects'
                },
                params: {
                    namespace: null
                }
            })
            .state('app.project', {
                abstract: true,
                url: 'projects/:projectId',
                views: {
                    main: {
                        template: '<ncl-project></ncl-project>'
                    }
                },
                params: {
                    createCancelled: false
                },
                data: {
                    pageTitle: 'Functions',
                    mainHeaderTitle: 'Functions'
                }
            })
            .state('app.project.functions', {
                url: '/functions',
                views: {
                    project: {
                        template: '<functions-data-wrapper></functions-data-wrapper>'
                    }
                },
                data: {
                    pageTitle: 'Functions',
                    mainHeaderTitle: 'Functions'
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
                    pageTitle: 'Create Function'
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
                    pageTitle: 'Functions',
                    mainHeaderTitle: 'Functions'
                },
                resolve: {
                    function: [
                        'FunctionsService', 'NuclioFunctionsDataService', 'NuclioProjectsDataService', '$state', '$stateParams',
                        function (FunctionsService, NuclioFunctionsDataService, NuclioProjectsDataService, $state, $stateParams) {
                            return NuclioProjectsDataService.getProject($stateParams.projectId).then(function (project) {
                                if ($stateParams.isNewFunction) {
                                    return angular.copy($stateParams.functionData);
                                } else {
                                    var functionMetadata = {
                                        name: $stateParams.functionId,
                                        namespace: project.metadata.namespace,
                                        projectName: project.metadata.name
                                    };

                                    return NuclioFunctionsDataService.getFunction(functionMetadata)
                                        .catch(function () {
                                            $state.go('app.project.functions', {projectId: $stateParams.projectId});
                                        });
                                }
                            })
                                .catch(function () {
                                    $state.go('app.projects');
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
                        template: '<version-data-wrapper data-version="$resolve.function"></version-data-wrapper>'
                    }
                },
                params: {
                    functionData: {}
                },
                data: {
                    pageTitle: 'Edit version',
                    mainHeaderTitle: 'Edit version'
                }
            })
            .state('app.project.function.edit.code', {
                url: '/code',
                views: {
                    version: {
                        template: '<ncl-version-code data-version="$resolve.function"></ncl-version-code>'
                    }
                },
                params: {
                    functionData: {}
                },
                data: {
                    pageTitle: 'Code'
                }
            })
            .state('app.project.function.edit.configuration', {
                url: '/configuration',
                views: {
                    version: {
                        template: '<ncl-version-configuration data-version="$resolve.function"></ncl-version-configuration>'
                    }
                },
                params: {
                    functionData: {}
                },
                data: {
                    pageTitle: 'Configuration'
                }
            })
            .state('app.project.function.edit.triggers', {
                url: '/triggers',
                views: {
                    version: {
                        template: '<ncl-version-triggers data-version="$resolve.function"></ncl-version-triggers>'
                    }
                },
                params: {
                    functionData: {}
                },
                data: {
                    pageTitle: 'Triggers'
                }
            })
            .state('app.project.function.edit.monitoring', {
                url: '/monitoring',
                views: {
                    version: {
                        template: '<ncl-version-monitoring data-version="$resolve.function"></ncl-version-monitoring>'
                    }
                },
                params: {
                    functionData: {}
                },
                data: {
                    pageTitle: 'Monitoring'
                }
            });

        $urlRouterProvider
            .when('/projects/:id', '/projects/:id/functions')
            .when('/control-panel', '/control-panel/logs')
            .when('/storage-pools/:id', '/storage-pools/:id/overview')
            .when('/projects/:id', '/projects/:id/functions')
            .when('/projects/:id/functions/:functionId', '/projects/:id/functions/:functionId/code')

            .otherwise(function ($injector) {
                $injector.get('$state').go('app.projects');
            });
    }
}());

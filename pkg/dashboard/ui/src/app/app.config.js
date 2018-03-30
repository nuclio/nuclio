(function () {
    'use strict';

    var injectedConfig = angular.fromJson('/* @echo IGZ_CUSTOM_CONFIG */' || '{}');

    var defaultConfig = {
        mode: 'production',
        isDemoMode: function () {
            return defaultConfig.mode === 'demo';
        },
        isStagingMode: function (strict) {
            return defaultConfig.mode === 'staging' || !strict && defaultConfig.mode === 'demo';
        }
    };

    angular.module('iguazio.app')
        .constant('ConfigService', window._.merge(defaultConfig, injectedConfig));

    angular.module('iguazio.app')
        .config(config)
        .config(ngDialogConfig)
        .config(scrollBarsConfig);

    function config($compileProvider, $locationProvider, $httpProvider, $qProvider) {
        $locationProvider.html5Mode(true);

        $httpProvider.defaults.withCredentials = true;

        // allows to get values from bindings outside of onInit function (since angular 1.6)
        $compileProvider.preAssignBindingsEnabled(true);

        // prevents 'Possibly unhandled rejection' error
        $qProvider.errorOnUnhandledRejections(false);
    }

    function ngDialogConfig(ngDialogProvider) {
        ngDialogProvider.setDefaults({
            className: 'ngdialog-theme-nuclio',
            showClose: false,
            closeByEscape: false,
            closeByDocument: false
        });
    }

    function scrollBarsConfig(ScrollBarsProvider) {
        ScrollBarsProvider.defaults = {
            scrollButtons: {
                enable: false // disable scrolling buttons by default
            },
            axis: 'y', // enable 1 axis scrollbar by default
            autoHideScrollbar: false,
            alwaysShowScrollbar: 0,
            theme: 'dark'
        };
    }
}());

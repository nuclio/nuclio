/*
Copyright 2023 The Nuclio Authors.

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

    angular.module('nuclio.app', [
        'nuclio.app.templates',
        'ui.router',
        'ui.layout',
        'ui.bootstrap',
        'ngAnimate',
        'ngCookies',
        'ngSanitize',
        'angular-yamljs',
        'angular-moment',
        'angular-base64',
        'ngDialog',
        'ngScrollbars',
        'fiestah.money',
        'dibari.angular-ellipsis',
        'restangular',
        'iguazio.dashboard-controls',
        'ngFileUpload',
        'rzModule',
        'angular-cron-jobs',
        'jm.i18next'
    ]);
}());

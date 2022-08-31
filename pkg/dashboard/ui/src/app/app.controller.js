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
        .controller('AppController', AppController);

    function AppController($transitions, $i18next, i18next) {
        var ctrl = this;
        var lng = i18next.language;

        activate();

        function activate() {
            $transitions.onSuccess({}, function (event) {
                var toState = event.$to();
                if (angular.isDefined(toState.data.pageTitle)) {
                    ctrl.pageTitle = $i18next.t(toState.data.pageTitle, {lng: lng}) + ' | Nuclio';
                }
            });
        }
    }
}());

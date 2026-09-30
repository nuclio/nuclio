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

    angular.module('nuclio.app')
        .factory('LoginService', LoginService);

    // components bundled from iguazio.dashboard-controls (e.g. version-execution-log) expect a
    // "LoginService" with an isLoggedIn() check (used to gate auto-refresh polling). This app has
    // no session-based login concept of its own - basic-auth (when enabled) is enforced by the
    // browser/server before any request succeeds, so there is no separate client-side "logged out"
    // state to represent.
    function LoginService() {
        return {
            isLoggedIn: isLoggedIn
        };

        function isLoggedIn() {
            return true;
        }
    }
}());

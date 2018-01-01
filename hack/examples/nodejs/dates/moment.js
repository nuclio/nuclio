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

var moment = require('moment');

exports.handler = function(context, event) {
    var request = JSON.parse(event.body.toString()); // event.body is a Buffer
    var now = moment();

    context.log_info('adding: ' + request.quantity + request.unit + ' to ' + now.format());

    now.add(request.quantify, request.unit);
    context.callback(now.format());
}

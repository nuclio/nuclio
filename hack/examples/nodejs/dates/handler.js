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

// Uses moment.js (installed as part of the build) to add a given amount of time
// to "now", and returns this as string. Invoke with a JSON containing:
//  - value: some number
//  - unit: some momentjs unit, as a string - e.g. days, d, hours, miliseconds
//
// For example, the following will add 3 hours to current time and return the response:
// {
//     "value": 3,
//     "unit": "hours"
// }
//

var moment = require('moment');

exports.handler = function(context, event) {
    var request = JSON.parse(event.body);
    var now = moment();

    context.logger.infoWith('Adding to now', {
        'request': request,
        'to': now.format()
    });

    now.add(request.value, request.unit);

    context.callback(now.format());
};

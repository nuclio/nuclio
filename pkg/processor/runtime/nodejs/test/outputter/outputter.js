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

// We use non default file name and handler name to test configuration as well
exports.testHandler = function(context, event) {
    if (event.method != 'POST') {
        context.callback(event.method);
	return;
    }

    var body = event.body.toString();

    switch (body) {
	case 'return_string':
	    context.callback('a string');
	    return;
	case 'return_status_and_string':
	    context.callback([201, 'a string after status']);
	    return;
	case 'return_list':
	    context.callback([{a: 1}, {b: 2}]);
	    return;
	case 'return_status_and_dict':
	    context.callback([201, {a: 'dict after status', b: 'foo'}]);
	    return;
	case 'log':
	    context.logger.debug('Debug message');
	    context.logger.info('Info message');
	    context.logger.warn('Warn message');
	    context.logger.error('Error message');
	    context.callback([201, 'returned logs']);
	    return;
	case 'log_with':
	    context.logger.errorWith('Error message', {source: 'rabbit', weight: 7});
	    context.callback([201, 'returned logs with']);
	    return;
	case 'return_response':
	    headers = event.headers;
	    headers['h1'] = 'v1';
	    headers['h2'] = 'v2';

	    context.callback(new context.Response('response body', headers, 'text/plain', 201));
	    return;
	case 'return_fields':
	    var fields = [];
	    for (var key in event.fields) {
		fields.push(key + '=' + event.fields[key]);
	    }
	    // We use sorted to get predictable output
	    fields.sort();
	    context.callback(fields.join(','));
	    return;
	case 'return_path':
	    context.callback(event.path);
	    return;
	case 'return_error':
	    throw 'some error';
	default:
	    throw 'Unknown return mode: ' + event.body;
    }
}

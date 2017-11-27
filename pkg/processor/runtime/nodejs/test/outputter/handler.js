function handler(context, event) {
    if (event.method != 'POST') {
        return event.method;
    }

    switch (event.body) {
	case 'return_string':
	    return 'a string'
	case 'return_status_and_string':
	    return [201, 'a string after status'];
	case 'return_list':
	    return [{'a': 1}, {'b': 2}];
	case 'return_status_and_dict':
	    return [201, {a: 'dict after status', b: 'foo'}];
	case 'log':
	    context.log_debug('Debug message');
	    context.log_info('Info message');
	    context.log_warn('Warn message');
	    context.log_error('Error message');
	    return [201, 'returned logs'];
	case 'log_with':
	    context.log_error_with('Error message', {source: 'rabbit', weight: 7});
	    return [201, 'returned logs with'];
	case 'return_response':
	    headers = event.headers;
	    headers['h1'] = 'v1';
	    headers['h2'] = 'v2';

	    return {
		body: 'response body',
		headers: headers,
		content_type: 'text/plain',
		status_code:201
	    };
	case 'return_fields':
	    var fields = [];
	    for (var key in event.fields) {
		fields.push(key + '=' + event.fields[key]);
	    }
	    // We use sorted to get predictable output
	    fields.sort();
	    return fields.join(',');
	case 'return_path':
	    return event.path;
	case 'return_error':
	    throw 'some error';
	default:
	    throw 'Unknown return mode: ' + event.body;
    }
}

package python3

/* Package python3 provides embedded Python 3 runtime

You need to write a handler function that gets a context and an event

	def handler(context, event):
		# Your code goes here


The handler can return:
- None
- bytes/string
- tuple of (status_code, bytes/string)
- nuclio.Response object


nuclio.Context
	logger		nuclio.Logger instance
	Response	nuclio.Response class (for creating responses)


nuclio.Logger
	error(message)
	warning(message)
	info(message)
	debug(message)
	error_with(message, **kw)
	warning_with(message, **kw)
	info_with(message, **kw)
	debug_with(message, **kw)

	Notes:
		- message must be a str
		- **kw keys must be strings and values can be str, int or float

nuclio.Event
    id				str
    trigger			nuclio.TriggerInfo
    content_type    str
    body			bytes
    headers			dict
    fields			dict
    timestamp		datetime
    path			str
    url				str
    method			str
    shard_id		int
    num_shards		int

nuclio.TriggerInfo
	klass	str
	king	str


nuclio.Response
	body			bytes
	status_code		int
	content_type	str
	headers			dict

	Create:
		conetxt.Response(body=b'', status_code=200, content_type='text/plain', headers=None)

	Notes:
		- You can pass str to body and it'll be encoded with UTF-8 encoding
		- headers keys must be strings, values can be str, int or float
*/

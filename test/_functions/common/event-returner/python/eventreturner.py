import json

def handler(context, event):

    # for object bodies, just take it as is. otherwise decode
    if not isinstance(event.body, dict):
        body = event.body.decode('utf8')
    else:
        body = event.body

    return json.dumps({
        'id': event.id,
        'triggerClass': event.trigger.klass,
        'eventType': event.trigger.kind,
        'contentType': event.content_type,
        'headers': dict(event.headers),
        'timestamp': event.timestamp.isoformat('T') + 'Z',
        'path': event.path,
        'url': event.url,
        'method': event.method,
        'type': event.type,
        'typeVersion': event.type_version,
        'version': event.version,
        'body': body
    })

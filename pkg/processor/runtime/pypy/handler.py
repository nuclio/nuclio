def event_handler(context, event):
    print('EVENT')
    print('VERSION', event.version())
    print('ID', event.id())
    print('SIZE ', event.size())
    print('TCLASS', event.trigger_class())
    print('TKIND', event.trigger_kind())
    print('CTYPE', event.content_type())
    print('BODY', event.body().decode('utf-8'))
    #print('HDR', event.header('key1'))
    print('TS', event.timestamp())
    print('PATH', event.path().decode('utf-8'))
    print('URL', event.url().decode('utf-8'))
    print('METHOD', event.method().decode('utf-8'))

    return 'RETURN VALUE'

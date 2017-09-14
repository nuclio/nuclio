def reverser(context, event):
    """Return reversed body as string"""

    context.logger.info('Hello from Python')
    body = event.body.decode('utf-8')
    return body[::-1]

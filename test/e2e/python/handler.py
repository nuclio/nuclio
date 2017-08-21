def reverser(event):
    """Return reversed body as string"""
    body = event.body.decode('utf-8')
    return body[::-1]

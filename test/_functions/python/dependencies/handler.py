import requests


def handler(context, event):
    return requests.__version__

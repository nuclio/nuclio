#!/usr/bin/env python

import os
import subprocess
import argparse
import re

minikube_ip_addr = ''


def _log(message):
    print '> ' + message


def _run_command(cmd):
    _log('Running: "{0}"'.format(cmd))
    out = subprocess.check_output(cmd.split(' '))
    _log('Result: {0}'.format(out))

    return out


def _run_minikube_cmd(cmd):
    _log('Running in minikube: "{0}"'.format(cmd))
    out = subprocess.check_output(['minikube', 'ssh', '--'] + cmd.split(' '))
    _log('Result: {0}'.format(out))

    return out


def _push_image(image_url):
    _log('Pushing {0}'.format(image_url))

    image_name = image_url.split('/')[1]
    minikube_image_url = '{0}/{1}'.format(minikube_ip_addr, image_name)
    localhost_image_url = 'localhost:5000/' + image_name

    # push to minikube
    _run_command('docker tag {0} {1}'.format(image_url, minikube_image_url))
    _run_command('docker push {0}'.format(minikube_image_url))
    _run_command('docker rmi {0}'.format(minikube_image_url))

    # ask minikube to pull and tag as nuclio/
    _run_minikube_cmd('docker pull {0}'.format(localhost_image_url))
    _run_minikube_cmd('docker tag {0} {1}'.format(localhost_image_url, image_url))
    _run_minikube_cmd('docker rmi {0}'.format(localhost_image_url))


if __name__ == '__main__':
    arg_parser = argparse.ArgumentParser()

    # name regex
    arg_parser.add_argument('--name', default='', help='regex pattern to match against image names')

    # parse the args
    args = arg_parser.parse_args()

    # create name matcher. if it's empty, it'll match all
    name_matcher = re.compile(args.name)

    minikube_ip_addr = _run_command('minikube ip').strip() + ':5000'

    tag = '{0}-amd64'.format(os.environ.get('NUCLIO_LABEL', 'latest'))

    for image_url in [
        'nuclio/controller:' + tag,
        'nuclio/playground:' + tag,
        'nuclio/dashboard:' + tag,
        'nuclio/handler-builder-golang-onbuild:' + tag,
        'nuclio/handler-builder-golang-onbuild:' + tag + '-alpine',
        'nuclio/handler-builder-java-onbuild:' + tag,
        'nuclio/handler-builder-dotnetcore-onbuild:' + tag,
        'nuclio/handler-builder-nodejs-onbuild:' + tag,
        'nuclio/handler-builder-python-onbuild:' + tag
    ]:
        if name_matcher.search(image_url):
            _push_image(image_url)

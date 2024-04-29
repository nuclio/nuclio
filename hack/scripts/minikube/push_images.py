#!/usr/bin/env python3
# Copyright 2023 The Nuclio Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

import os
import subprocess
import argparse
import re

minikube_ip_addr = ''


def _log(message):
    print('> ' + message)


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
    name_matcher = re.compile(re.escape(args.name))

    minikube_ip_addr = '{0}:5000'.format(_run_command('minikube ip').strip().decode())

    tag = '{0}-amd64'.format(os.environ.get('NUCLIO_LABEL', 'latest'))

    for image_url in [
        'nuclio/controller:' + tag,
        'nuclio/dashboard:' + tag,
        'nuclio/dlx:' + tag,
        'nuclio/autoscaler:' + tag,
        'nuclio/handler-builder-golang-onbuild:' + tag,
        'nuclio/handler-builder-golang-onbuild:' + tag + '-alpine',
        'nuclio/handler-builder-java-onbuild:' + tag,
        'nuclio/handler-builder-dotnetcore-onbuild:' + tag,
        'nuclio/handler-builder-nodejs-onbuild:' + tag,
        'nuclio/handler-builder-python-onbuild:' + tag
    ]:
        if name_matcher.search(image_url):
            _push_image(image_url)

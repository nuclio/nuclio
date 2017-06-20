#!/usr/bin/env python

import argparse, subprocess, os

NUCLIO_BUILDER_OUTPUT_NAME = 'nuclio-builder-output'
NUCLIO_BUILDER_CONTAINER = 'nuclio/builder-output'

BUILD_LOG = open('nuclio.build.log', 'w')


def _get_work_path():
    return os.path.dirname(os.path.realpath(__file__))


def _run_shell(cmd, wait=True, **kwargs):
    if wait:
        subprocess.check_call(cmd, shell=True, stderr=subprocess.STDOUT, stdout=BUILD_LOG, **kwargs)
    else:
        subprocess.call(cmd, shell=True, stderr=subprocess.STDOUT, stdout=BUILD_LOG, **kwargs)


def _build_on_build():
    print('Preparing onbuild container')
    _run_shell('docker build --tag nuclio/nuclio:onbuild .',
               cwd=os.path.join(_get_work_path(), 'docker'))


def _build():
    print('Building Nuclio')
    _run_shell('docker build --tag {0} --file {1}/Dockerfile .'.format(NUCLIO_BUILDER_CONTAINER,
                                                                       _get_work_path()))


def _copy_binaries():
    _run_shell('docker rm -f {0}'.format(NUCLIO_BUILDER_OUTPUT_NAME), wait=False)
    _run_shell('rm -rf bin', wait=False)
    _run_shell('mkdir -p bin', wait=False)
    _run_shell('docker run --name {0} {1}'.format(NUCLIO_BUILDER_OUTPUT_NAME, NUCLIO_BUILDER_CONTAINER))
    _run_shell('docker cp {0}:/go/bin/processor bin/'.format(NUCLIO_BUILDER_OUTPUT_NAME))
    _run_shell('docker rm -f {0}'.format(NUCLIO_BUILDER_OUTPUT_NAME))


def _create_image(dockerfile):
    print('Creating Nuclio docker image')
    _run_shell(
        'docker build --squash --tag nuclio/nuclio:latest --file {0} .'.format(os.path.join(_get_work_path(),
                                                                                            'docker',
                                                                                            dockerfile)))


def main():
    parser = argparse.ArgumentParser(description='Build Nuclio Processor', prog='build.py')
    parser.add_argument('--output', '-o',
                        choices=['docker', 'binary'],
                        default='docker',
                        nargs='?',
                        help='Build output type (default: docker)')
    parser.add_argument('--deps', '-d',
                        type=str,
                        nargs='?',
                        help='Builder dependencies (for apt-get command)')

    args = parser.parse_args()

    if args.deps:
        subprocess.call('cp {0} .deps'.format(args.deps))

    _build_on_build()
    _build()
    _copy_binaries()

    if args.output == 'docker':
        dockerfile = 'Dockerfile.alpine'
        if args.deps:
            dockerfile = 'Dockerfile.jessie'
        _create_image(dockerfile)
        print('Nuclio\'s docker ready and labeled \'nuclio/nuclio\'')
    else:
        print(
            'Nuclio\'s processor binary is located at {0}'.format(
                os.path.abspath(os.path.join(_get_work_path(), '..', '..', '..', 'bin'))))


if __name__ == '__main__':
    main()

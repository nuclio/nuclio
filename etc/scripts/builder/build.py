#!/usr/bin/env python

import argparse, subprocess, os

NUCLIO_BUILDER_OUTPUT_NAME = 'nuclio-builder-output'
NUCLIO_BUILDER_CONTAINER = 'nuclio/builder-output'

BUILD_LOG = open('nuclio.build.log', 'w')


def __build_on_build():
    print('preparing onbuild container')
    subprocess.check_call('docker build --tag nuclio/nuclio:onbuild .', shell=True,
                          cwd=os.path.join(os.getcwd(), 'etc', 'scripts', 'builder', 'docker'),
                          stderr=subprocess.STDOUT, stdout=BUILD_LOG)


def __build():
    print('building Nuclio')
    subprocess.check_call('docker build --tag {0} --file {1}/Dockerfile .'.format(NUCLIO_BUILDER_CONTAINER,
                                                                                  os.path.join(os.getcwd(),
                                                                                               'etc',
                                                                                               'scripts',
                                                                                               'builder')), shell=True,
                          stderr=subprocess.STDOUT, stdout=BUILD_LOG)


def __copy_binaries():
    subprocess.call('docker rm -f {0}'.format(NUCLIO_BUILDER_OUTPUT_NAME), shell=True,
                    stderr=subprocess.STDOUT, stdout=BUILD_LOG)
    subprocess.call('rm -rf bin', shell=True,
                    stderr=subprocess.STDOUT, stdout=BUILD_LOG)
    subprocess.call('mkdir -p bin', shell=True,
                    stderr=subprocess.STDOUT, stdout=BUILD_LOG)
    subprocess.check_call('docker run --name {0} {1}'.format(NUCLIO_BUILDER_OUTPUT_NAME, NUCLIO_BUILDER_CONTAINER),
                          shell=True,
                          stderr=subprocess.STDOUT, stdout=BUILD_LOG)
    subprocess.check_call('docker cp {0}:/go/bin/processor bin/'.format(NUCLIO_BUILDER_OUTPUT_NAME), shell=True,
                          stderr=subprocess.STDOUT, stdout=BUILD_LOG)
    subprocess.check_call('docker rm -f {0}'.format(NUCLIO_BUILDER_OUTPUT_NAME), shell=True,
                          stderr=subprocess.STDOUT, stdout=BUILD_LOG)


def __create_docker(dockerfile):
    print('creating Nuclio docker image')
    subprocess.check_call(
        'docker build --squash --tag nuclio/nuclio:latest --file {0} .'.format(os.path.join(os.getcwd(),
                                                                                            'etc',
                                                                                            'scripts',
                                                                                            'builder',
                                                                                            'docker',
                                                                                            dockerfile)),
        shell=True,
        stderr=subprocess.STDOUT, stdout=BUILD_LOG)


def main():
    parser = argparse.ArgumentParser(description='Build Nuclio', prog='build.py')
    parser.add_argument('--output', '-O',
                        choices=['docker', 'binary'],
                        default='docker',
                        nargs='?',
                        help='Build output type (default: docker)')
    parser.add_argument('--deps', '-D',
                        type=str,
                        nargs='?',
                        help='Builder dependencies (for apt-get command)')

    args = parser.parse_args()

    if args.deps:
        subprocess.call('cp {0} .deps'.format(args.deps))

    __build_on_build()
    __build()
    __copy_binaries()

    if args.output == 'docker':
        dockerfile = 'Dockerfile.alpine'
        if args.deps:
            dockerfile = 'Dockerfile.jessie'
        __create_docker(dockerfile)
        print('Nuclio\'s docker ready and labeled \'nuclio/nuclio\'')
    else:
        print('Nuclio\'s processor binary is located at {0}'.format(os.path.join(os.getcwd(), 'bin')))


if __name__ == '__main__':
    main()

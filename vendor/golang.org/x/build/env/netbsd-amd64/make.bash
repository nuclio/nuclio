#!/bin/bash
# Copyright 2016 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# This script uses Anita (an automated NetBSD installer) for setting up
# the VM. It needs the following things on the build host:
#  - qemu
#  - cdrtools
#  - GNU tar (not BSD tar)
#  - Python 2.7
#  - python-pexpect

set -e -x

ANITA_VERSION=1.44
ARCH=amd64
# The release that the packages have been built for.
RELEASE=8.0_2018Q1

# Must use GNU tar. On NetBSD, tar is BSD tar and gtar is GNU.
TAR=tar
if which gtar > /dev/null; then
  TAR=gtar
fi

WORKDIR=work-NetBSD-${ARCH}

# Remove WORKDIR unless -k (keep) is given.
if [ "$1" != "-k" ]; then
  rm -rf ${WORKDIR}
fi

# Download and build anita (automated NetBSD installer).
if ! sha1sum -c anita-${ANITA_VERSION}.tar.gz.sha1; then
  curl -vO http://www.gson.org/netbsd/anita/download/anita-${ANITA_VERSION}.tar.gz
  sha1sum -c anita-${ANITA_VERSION}.tar.gz.sha1 || exit 1
fi

tar xfz anita-${ANITA_VERSION}.tar.gz
cd anita-${ANITA_VERSION}
python setup.py build
cd ..

env PYTHONPATH=${PWD}/anita-${ANITA_VERSION} python mkvm.py ${ARCH} ${RELEASE}

echo "Archiving wd0.img (this may take a while)"
${TAR} -Szcf netbsd-${ARCH}-${RELEASE}.tar.gz --transform s,${WORKDIR}/wd0.img,disk.raw, ${WORKDIR}/wd0.img
echo "Done. GCE image is netbsd-${ARCH}-${RELEASE}.tar.gz."

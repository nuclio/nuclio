#!/usr/bin/env python
# Copyright 2016 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

import anita
import ftplib
import sys

def find_latest_release(branch, arch):
  """Find the latest NetBSD-current release for the given arch.

  Args:
    branch: the NetBSD branch (e.g. HEAD, netbsd-8).
    arch: the architecture (e.g. amd64).

  Returns:
    the full path to the release.
  """
  conn = ftplib.FTP('nyftp.netbsd.org')
  conn.login()
  conn.cwd('/pub/NetBSD-daily/%s' % branch)
  releases = conn.nlst()
  releases.sort(reverse=True)
  for r in releases:
    archs = conn.nlst(r)
    if not archs:
      next
    has_arch = [a for a in archs if a.endswith(arch)]
    if has_arch:
      return "https://nyftp.netbsd.org/pub/NetBSD-daily/%s/%s/" % (branch, has_arch[0])


arch = sys.argv[1]
release = sys.argv[2]

commands = [
    """cat >> /etc/rc.local <<EOF
(
  export PATH=/usr/pkg/bin:/usr/pkg/sbin:${PATH}
  export GOROOT_BOOTSTRAP=/usr/pkg/go14
  set -x
  echo 'starting buildlet script'
  netstat -rn
  cat /etc/resolv.conf
  dig metadata.google.internal
  (
    set -e
    curl -o /buildlet \$(curl -H 'Metadata-Flavor: Google' http://metadata.google.internal/computeMetadata/v1/instance/attributes/buildlet-binary-url)
    chmod +x /buildlet
    exec /buildlet
  )
  echo 'giving up'
  sleep 10
  halt -p
)
EOF""",
    """cat > /etc/ifconfig.vioif0 << EOF
!dhcpcd
mtu 1460
EOF""",
    "dhcpcd",
    "env PKG_PATH=http://ftp.netbsd.org/pub/pkgsrc/packages/NetBSD/%s/%s/All/ pkg_add bash curl" % (arch, release),
    "env PKG_PATH=http://ftp.netbsd.org/pub/pkgsrc/packages/NetBSD/%s/%s/All/ pkg_add git-base" % (arch, release),
    "env PKG_PATH=http://ftp.netbsd.org/pub/pkgsrc/packages/NetBSD/%s/%s/All/ pkg_add mozilla-rootcerts go14" % (arch, release),
    # Interactive debugging tools for users using gomote ssh:
    "env PKG_PATH=http://ftp.netbsd.org/pub/pkgsrc/packages/NetBSD/%s/%s/All/ pkg_add emacs25-nox11 vim screen" % (arch, release),
    # For https://golang.org/issue/24354
    "env PKG_PATH=http://ftp.netbsd.org/pub/pkgsrc/packages/NetBSD/%s/%s/All/ pkg_add clang cmake" % (arch, release),
    """ed /etc/fstab << EOF
H
%s/wd0/sd0/
wq
EOF""",
    "touch /etc/openssl/openssl.cnf",
    "/usr/pkg/sbin/mozilla-rootcerts install",
    "echo sshd=yes >> /etc/rc.conf",
    "echo PermitRootLogin without-password >> /etc/ssh/sshd_config",
    "/etc/rc.d/sshd restart",
    "sync; shutdown -hp now",
]


a = anita.Anita(
    anita.URL('https://cdn.netbsd.org/pub/NetBSD/NetBSD-8.0_RC1/%s/' % arch),
    workdir="work-NetBSD-%s" % arch,
    disk_size="4G",
    memory_size = "1G",
    persist=True)
child = a.boot()
anita.login(child)

for cmd in commands:
  anita.shell_cmd(child, cmd, 3600)

# Sometimes, the halt command times out, even though it has completed
# successfully.
try:
    a.halt()
except:
    pass

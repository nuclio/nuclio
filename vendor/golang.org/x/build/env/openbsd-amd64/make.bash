#!/bin/bash
# Copyright 2014 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# This script requires expect, growisofs and qemu.

set -e
set -u

readonly VERSION="6.4"
readonly RELNO="${VERSION/./}"
readonly SNAPSHOT=false

readonly ARCH="${ARCH:-amd64}"
readonly MIRROR="${MIRROR:-ftp.usa.openbsd.org}"

if [[ "${ARCH}" != "amd64" && "${ARCH}" != "i386" ]]; then
  echo "ARCH must be amd64 or i386"
  exit 1
fi

readonly ISO="install${RELNO}-${ARCH}.iso"
readonly ISO_PATCHED="install${RELNO}-${ARCH}-patched.iso"

if [[ ! -f "${ISO}" ]]; then
  DIR="${VERSION}"
  if [[ "$SNAPSHOT" = true ]]; then
    DIR="snapshots"
  fi
  curl -o "${ISO}" "https://${MIRROR}/pub/OpenBSD/${DIR}/${ARCH}/install${RELNO}.iso"
fi

function cleanup() {
	rm -f "${ISO_PATCHED}"
	rm -f auto_install.conf
	rm -f boot.conf
	rm -f disk.raw
	rm -f disklabel.template
	rm -f etc/{installurl,rc.local,sysctl.conf}
	rm -f install.site
	rm -f random.seed
	rm -f site${RELNO}.tgz
	rmdir etc
}

trap cleanup EXIT INT

# XXX: Download and save bash, curl, and their dependencies too?
# Currently we download them from the network during the install process.

# Create custom siteXX.tgz set.
PKG_ADD_OPTIONS=""
if [[ "$SNAPSHOT" = true ]]; then
  PKG_ADD_OPTIONS="-D snap"
fi
mkdir -p etc
cat >install.site <<EOF
#!/bin/sh
syspatch
pkg_add -iv ${PKG_ADD_OPTIONS} bash curl git

echo 'set tty com0' > boot.conf
EOF

cat >etc/installurl <<EOF
https://${MIRROR}/pub/OpenBSD
EOF
cat >etc/rc.local <<EOF
(
  set -x

  echo "Remounting root with softdep,noatime..."
  mount -o softdep,noatime,update /

  echo "starting buildlet script"
  netstat -rn
  cat /etc/resolv.conf
  dig metadata.google.internal
  (
    set -e
    export PATH="\$PATH:/usr/local/bin"
    /usr/local/bin/curl -o /buildlet \$(/usr/local/bin/curl --fail -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/attributes/buildlet-binary-url)
    chmod +x /buildlet
    exec /buildlet
  )
  echo "giving up"
  (
    sleep 10
    halt -p
  )&
)
EOF
cat >etc/sysctl.conf <<EOF
hw.smt=1
EOF
chmod +x install.site
tar -zcvf site${RELNO}.tgz install.site etc/{installurl,rc.local,sysctl.conf}

# Autoinstall script.
cat >auto_install.conf <<EOF
System hostname = buildlet
Which network interface = vio0
IPv4 address for vio0 = dhcp
IPv6 address for vio0 = none
Password for root account = root
Do you expect to run the X Window System = no
Change the default console to com0 = yes
Which speed should com0 use = 115200
Setup a user = gopher
Full name for user gopher = Gopher Gopherson
Password for user gopher = gopher
Allow root ssh login = no
What timezone = US/Pacific
Which disk = sd0
Use (W)hole disk or (E)dit the MBR = whole
Use (A)uto layout, (E)dit auto layout, or create (C)ustom layout = auto
URL to autopartitioning template for disklabel = file://disklabel.template
Set name(s) = +* -x* -game* -man* done
Directory does not contain SHA256.sig. Continue without verification = yes
EOF

# Disklabel template.
cat >disklabel.template <<EOF
/	5G-*	95%
swap	1G
EOF

# Hack install CD a bit.
echo 'set tty com0' > boot.conf
dd if=/dev/urandom of=random.seed bs=4096 count=1
cp "${ISO}" "${ISO_PATCHED}"
growisofs -M "${ISO_PATCHED}" -l -R -graft-points \
  /${VERSION}/${ARCH}/site${RELNO}.tgz=site${RELNO}.tgz \
  /auto_install.conf=auto_install.conf \
  /disklabel.template=disklabel.template \
  /etc/boot.conf=boot.conf \
  /etc/random.seed=random.seed

# Initialize disk image.
rm -f disk.raw
qemu-img create -f raw disk.raw 10G

# Run the installer to create the disk image.
expect <<EOF
set timeout 1800

spawn qemu-system-x86_64 -nographic -smp 2 \
  -drive if=virtio,file=disk.raw,format=raw -cdrom "${ISO_PATCHED}" \
  -net nic,model=virtio -net user -boot once=d

expect timeout { exit 1 } "boot>"
send "\n"

# Need to wait for the kernel to boot.
expect timeout { exit 1 } "\(I\)nstall, \(U\)pgrade, \(A\)utoinstall or \(S\)hell\?"
send "s\n"

expect timeout { exit 1 } "# "
send "mount /dev/cd0c /mnt\n"
send "cp /mnt/auto_install.conf /mnt/disklabel.template /\n"
send "chmod a+r /disklabel.template\n"
send "umount /mnt\n"
send "exit\n"

expect timeout { exit 1 } "CONGRATULATIONS!"

# There is some form of race condition with OpenBSD 6.2 MP
# and qemu, which can result in init(1) failing to run /bin/sh
# the first time around...
expect {
  timeout { exit 1 }
  "Enter pathname of shell or RETURN for sh:" {
    send "\nexit\n"
    expect timeout { exit 1 } eof
  }
  eof
}
EOF

# Create Compute Engine disk image.
echo "Archiving disk.raw... (this may take a while)"
tar -Szcf "openbsd-${ARCH}-gce.tar.gz" disk.raw

echo "Done. GCE image is openbsd-${ARCH}-gce.tar.gz."

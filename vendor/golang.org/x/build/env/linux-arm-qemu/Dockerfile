# Copyright 2014 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# Base builder image: gobuilders/linux-arm-qemu

# Using sid for the cross toolchain.
FROM gobuilders/linux-x86-sid
MAINTAINER golang-dev <golang-dev@googlegroups.com>

ENV DEBIAN_FRONTEND noninteractive

RUN dpkg --add-architecture armhf
RUN apt-get update
RUN apt-get install -y --no-install-recommends qemu
# To build the ARM root.
RUN apt-get install -y --no-install-recommends debootstrap
# To build 5g & buildlet.
RUN apt-get install -y --no-install-recommends gcc git libc6-dev
# To build e2fsimage.
RUN apt-get install -y --no-install-recommends e2fslibs-dev
RUN apt-get install -y linux-source-3.16 xz-utils
RUN apt-get install -y gcc-arm-linux-gnueabihf

RUN mkdir /arm

# The list of packages in include are copied from linux-x86-std
RUN debootstrap --arch=armhf --foreign --include=curl,ca-certificates,strace,gcc,libc6-dev,gdb,lsof,psmisc sid /arm/root

# Script to finish off the debootstrap installation.
ADD stage2 /arm/root/stage2
RUN chmod +x /arm/root/stage2

# Setup networking.
RUN (echo "auto lo"; echo "iface lo inet loopback"; echo "auto eth0"; echo "iface eth0 inet dhcp") > /arm/root/etc/network/interfaces

# Run buildlet at boot.
ADD buildlet.service /arm/root/etc/systemd/system/buildlet.service
RUN (echo "[Journal]"; echo "ForwardToConsole=yes") > /arm/root/etc/systemd/journald.conf

# Fetch go1.4 and cross compile buildlet's stage0.
RUN mkdir /gopath
ENV GOPATH /gopath
ENV GOROOT /go1.4
ENV PATH $GOROOT/bin:$PATH
RUN cd $GOROOT/src && GOARCH=arm GOOS=linux ./make.bash
RUN GOARCH=arm GOOS=linux go get golang.org/x/build/cmd/buildlet/stage0
RUN mkdir -p /arm/root/usr/local/bin && cp $GOPATH/bin/linux_arm/stage0 /arm/root/usr/local/bin
RUN rm -rf /go1.4 /gopath

# Fetch go1.4.2 for arm
RUN curl -O http://dave.cheney.net/paste/go1.4.2.linux-arm~multiarch-armv7-1.tar.gz
RUN echo '607573c55dc89d135c3c9c84bba6ba6095a37a1e  go1.4.2.linux-arm~multiarch-armv7-1.tar.gz' | sha1sum -c
RUN tar xfv go1.4.2.linux-arm~multiarch-armv7-1.tar.gz -C /arm/root/
RUN rm go1.4.2.linux-arm~multiarch-armv7-1.tar.gz
RUN mv /arm/root/go /arm/root/go1.4
RUN rm -rf /arm/root/go1.4/api /arm/root/go1.4/blog /arm/root/go1.4/doc /arm/root/go1.4/misc /arm/root/go1.4/test
RUN find /arm/root/go1.4 -type d -name testdata | xargs rm -rf

# Install e2fsimage to prepare a root disk without running any
# "privilaged" docker operations (i.e. mount).
# Building from source because there aren't any Debian packages.
ENV IMG_SIZE 1000000 # in KB
RUN curl -L -O http://sourceforge.net/projects/e2fsimage/files/e2fsimage/0.2.2/e2fsimage-0.2.2.tar.gz
RUN echo '8ab6089c6a91043b251afc5c4331d1d740be1469  e2fsimage-0.2.2.tar.gz' | sha1sum -c
RUN tar xfv e2fsimage-0.2.2.tar.gz
# The configure script is broken. This is all we need anyway.
RUN cd e2fsimage-0.2.2/src && \
    gcc -o e2fsimage -DVER=\"0.2.2\" main.c copy.c symlink.c util.c mkdir.c dirent.c mke2fs.c inodb.c sfile.c uiddb.c uids.c malloc.c passwd.c group.c -lext2fs -lcom_err
RUN /e2fsimage-0.2.2/src/e2fsimage -f /arm/root.img -d /arm/root -s $IMG_SIZE -p
RUN rm -rf e2fsimage-0.2.2 e2fsimage-0.2.2.tar.gz
RUN rm -rf /arm/root

# Build a kernel. We're buildng here because we need a recent version for
# systemd to boot, and the binary ones in debian's repositories have a lot
# of needed components as modules (filesystem/sata drivers). It's just
# simpler to build a kernel than it is cross generate an initrd with
# the right bits in.
RUN tar xfv /usr/src/linux-source-3.16.tar.xz -C /usr/src/
COPY kernel_config /usr/src/linux-source-3.16/.config
RUN (cd /usr/src/linux-source-3.16 && \
     ARCH=arm CROSS_COMPILE=arm-linux-gnueabihf- make)
RUN cp /usr/src/linux-source-3.16/arch/arm/boot/zImage /arm/vmlinuz
RUN rm -rf /usr/src/linux-source-3.16

RUN qemu-system-arm -M vexpress-a9 -cpu cortex-a9 -nographic -no-reboot -sd /arm/root.img -kernel /arm/vmlinuz -append "root=/dev/mmcblk0 console=ttyAMA0 rw rootwait init=/stage2"

ADD buildlet-qemu /usr/local/bin/buildlet-qemu
RUN chmod +x /usr/local/bin/buildlet-qemu
ADD qemu.service /etc/systemd/system/qemu.service
RUN systemctl enable /etc/systemd/system/qemu.service
RUN systemctl disable /etc/systemd/system/buildlet.service

RUN rm /usr/local/bin/stage0
RUN apt-get purge -y gcc git libc6-dev xz-utils gcc-arm-linux-gnueabihf linux-source-3.16 e2fslibs-dev debootstrap strace gdb libc6-dev perl
RUN apt-get autoremove -y --purge
RUN apt-get clean
RUN rm -rf /var/lib/apt/lists /usr/share/doc
RUN (cd /usr/share/locale/ && ls -1 | grep -v en | xargs rm -rf)
RUN rm -rf /var/cache/debconf/*
RUN rm -rf /usr/share/man
RUN (cd /usr/bin && ls -1 | grep qemu | grep -v qemu-system-arm | xargs rm)

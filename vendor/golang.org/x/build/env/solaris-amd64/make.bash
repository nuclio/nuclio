#!/bin/bash
# Copyright 2016 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# The following variables need to be configured either
# below or externally.

if [ -z ${GOBUILDKEY+x} ]; then
    GOBUILDKEY="" # FILL ME IN
fi

if [ -z ${COORDINATOR+x} ]; then
    COORDINATOR="" # FILL ME IN
fi

if [ -z ${BUILDLET_NAME+x} ]; then
    BUILDLET_NAME="" # FILL ME IN
fi

######################################################

readonly BUILDLET_URL="https://storage.googleapis.com/go-builder-data/buildlet.solaris-amd64"
readonly BOOTSTRAP_URL="https://storage.googleapis.com/go-builder-data/gobootstrap-solaris-amd64.tar.gz"

# We need git to grab the source files and gcc for cgo.
pkg update
pkg install git gcc

# Get the bootstrapper.
mkdir -p /usr/local/go-bootstrap
(cd /usr/local/go-bootstrap && curl --silent $BOOTSTRAP_URL | tar xf -)
chown -R root:root /usr/local/go-bootstrap

# Set up the key.
echo $GOBUILDKEY > /root/.gobuildkey

# Write the startup script.
cat > /lib/svc/method/svc-buildlet <<EOF
#!/usr/sbin/sh
#
# Start method script for the go buildlet service.
#

. /lib/svc/share/smf_include.sh

if /usr/bin/pgrep -x -u 0 -z \`smf_zonename\` buildlet >/dev/null 2>&1; then
    echo "\$0: buildlet is already running"
    exit \$SMF_EXIT_ERR_NOSMF
fi

while true; do
    # Get the buildlet
    /usr/bin/curl --silent $BUILDLET_URL > /root/buildlet
    chmod +x /root/buildlet
    /root/buildlet -coordinator=$COORDINATOR:443 -reverse=$BUILDLET_NAME -halt=false 2>/dev/null
done &

exit \$SMF_EXIT_OK
EOF

# Make executable
chmod +x /lib/svc/method/svc-buildlet

# Write the service manifest.
cat > /lib/svc/manifest/site/buildlet.xml <<EOF
<?xml version="1.0"?>
<!DOCTYPE service_bundle SYSTEM "/usr/share/lib/xml/dtd/service_bundle.dtd.1">

<service_bundle type='manifest' name='golang-buildlet:buildlet'>

<service
    name='site/buildlet'
    type='service'
    version='1'>

    <create_default_instance enabled='true' />

    <single_instance/>

    <dependency
        name='usr'
        grouping='require_all'
        restart_on='none'
        type='service'>
        <service_fmri value='svc:/system/filesystem/minimal' />
    </dependency>

    <dependency
        name='network'
        grouping='require_all'
        restart_on='none'
        type='service'>
        <service_fmri value='svc:/milestone/network' />
    </dependency>

    <dependency
         name='rds_single'
         grouping='require_all'
         restart_on='none'
        type='service'>
        <service_fmri value='svc:/milestone/single-user' />
    </dependency>

    <method_context>
        <method_credential
            user='root'
            group='root'
            privileges='basic,net_icmpaccess,net_rawaccess' />
    </method_context>

    <exec_method
        type='method'
        name='start'
        exec='/lib/svc/method/svc-buildlet'
        timeout_seconds='60' >
    </exec_method>

    <exec_method
        type='method'
        name='stop'
        exec=':kill'
        timeout_seconds='60' >
    </exec_method>

    <property_group name="startd" type="framework">
        <propval name="ignore_error" type="astring" value="core,signal" />
    </property_group>

    <stability value='Evolving' />

    <template>
        <common_name>
            <loctext xml:lang='C'> Go Buildlet Service
            </loctext>
        </common_name>
    </template>
</service>

</service_bundle>
EOF

# Install the service.
svcadm restart manifest-import
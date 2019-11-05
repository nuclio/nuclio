#!/usr/bin/env sh

# TODO: Finish here and implement for go's onbuild

set +ex

cd /go/src/github.com/nuclio/handler

if [ ! -f "go.mod" ]; then
    cp /processor_go.mod ./go.mod
    cp /processor_go.sum ./go.sum
fi

if [ "${NUCLIO_BUILD_OFFLINE}" != "true" ]; then

    # online, user supplied his own vendor
    if [ -d "vendor" ]; then
        echo "TODO"
    else
        echo "TODO"
    fi
else
    # darksite, user supplied his own vendor
    if [ -d "vendor" ]; then
        rm -rf ./go.mod ./go.sum  # in case the exists, bye bye
        touch go.mod
        mv vendor vendor-temp
        mv /processor_vendor ./vendor
        cp -r vendor-temp/* ./vendor
        rm -rf vendor-temp
        echo "TODO"
    else
        # copy vendor dir from processor
        echo "TODO"
    fi
fi


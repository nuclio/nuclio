# Project: v3io-go

## Description:
**v3io-go** is an interface to V3IO API for Golang

## Prerequisites:
1. Captain proto compiler
    * MacOS: `brew install capnp`
    * Debian / Ubuntu: `apt-get install capnproto`
    * Other: Follow the guidelines https://capnproto.org/install.html

## Dependencies:
1. go-capnproto2
    * `git clone git@github.com:iguazio/go-capnproto2.git`

## Running the tests
1. **Define the following environment variables:**
    - V3IO_DATAPLANE_URL=http://<app-node>:8081
    - V3IO_DATAPLANE_USERNAME=<username>
    - V3IO_DATAPLANE_ACCESS_KEY=<access-key>
    - V3IO_CONTROLPLANE_URL=http://<data-node>:8001
    - V3IO_CONTROLPLANE_USERNAME=<admin-username>
    - V3IO_CONTROLPLANE_PASSWORD=<admin-password>
2. Run ``make test``
3. Alternatively you can pass environment variables inline as you can see in the following example: 
    ```
    V3IO_DATAPLANE_URL=http://<app-node>:8081 \
    V3IO_DATAPLANE_USERNAME=<username> \
    V3IO_DATAPLANE_ACCESS_KEY=<access-key> \
    V3IO_CONTROLPLANE_URL=http://<data-node>:8001 \
    V3IO_CONTROLPLANE_USERNAME=<admin-username> \
    V3IO_CONTROLPLANE_PASSWORD=<admin-password> \
    make test
    ```
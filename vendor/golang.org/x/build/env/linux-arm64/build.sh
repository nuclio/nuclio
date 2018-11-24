#!/bin/bash
#
# This is run on the arm64 host, with the Dockerfile in the same directory,
# by the build scripts in linaro and packet subdirectories.

docker build -t golang.org/linux-arm64 .
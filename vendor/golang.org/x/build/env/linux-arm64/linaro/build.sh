#!/bin/bash
#
# This is run on the arm64 host with the Dockerfile in the same directory. 
# The parent Dockerfile and build.sh (linux-arm64/*) must be in parent directory.

(cd ../ && ./build.sh) && docker build -t gobuilder-arm64-linaro:1 .
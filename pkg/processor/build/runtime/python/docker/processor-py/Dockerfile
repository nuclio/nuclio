FROM python:3-alpine3.6

COPY cmd/processor/_output/processor /usr/local/bin/processor
COPY pkg/processor/runtime/python/wrapper.py /opt/nuclio/wrapper.py

# generate a version file
ARG NUCLIO_VERSION_INFO_FILE_CONTENTS
RUN mkdir -p /etc/nuclio && echo ${NUCLIO_VERSION_INFO_FILE_CONTENTS} > /etc/nuclio/version_info.json

ARG NUCLIO_LABEL
ARG NUCLIO_DOCKER_REPO

FROM $NUCLIO_DOCKER_REPO/nuclio-base:$NUCLIO_LABEL

RUN apt update -qqy && apt install rsync -y

RUN git clone --branch release-1.23 https://github.com/kubernetes/code-generator.git /code-generator

CMD [ "./hack/scripts/generate-crds/update-codegen.sh" ]

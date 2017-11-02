# Copyright 2017 The Nuclio Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#	 http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

.PHOHY: error
all:
	go build .


.PHONY: instsall-linter
install-linter:
	@go get -u gopkg.in/alecthomas/gometalinter.v1
	@${GOPATH}/bin/gometalinter.v1 --install
	@go get -u github.com/pavius/impi/cmd/impi

.PHONY: lint
lint:
	@echo Verifying imports...
	@${GOPATH}/bin/impi \
	    --local github.com/nuclio/nuclio/ \
	    --scheme stdLocalThirdParty .

	@echo Linting...
	@${GOPATH}/bin/gometalinter.v1 \
		--disable-all \
		--enable=vet \
		--enable=vetshadow \
		--enable=deadcode \
		--enable=varcheck \
		--enable=staticcheck \
		--enable=gosimple \
		--enable=ineffassign \
		--enable=interfacer \
		--enable=unconvert \
		--enable=goconst \
		--enable=golint \
		--enable=misspell \
		--enable=gofmt \
		--enable=staticcheck \
		--exclude="_test.go" \
		--exclude="should have comment" \
		--exclude="comment on" \
		--exclude="error should be the last" \
		--deadline=300s \
		--concurrency 2 \
		.

	@echo Done.

.PHONY: test
test:
	go test -v .

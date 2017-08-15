all: controller nuclio-build nuclio-deploy
	@echo Done.

nuclio-build: ensure-gopath
	go build -o ${GOPATH}/bin/nuclio-build cmd/nuclio-build/main.go

nuclio-deploy: ensure-gopath
	go build -o ${GOPATH}/bin/nuclio-deploy cmd/nuclio-deploy/main.go

controller:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
	     go build -a -installsuffix cgo \
	     -o cmd/controller/_output/controller cmd/controller/main.go
	cd cmd/controller && docker build -t nuclio/controller .
	rm -rf cmd/controller/_output

.PHONY: get-sdk
get-sdk:
	go get github.com/nuclio/nuclio-sdk/...

.PHONY: test
test:
	go test -v ./cmd/...
	go test -v ./pkg/...

.PHONY: travis
travis: test
	go test -v test/e2e_test.go

.PHONY: ensure-gopath
check-gopath:
ifndef GOPATH
    $(error GOPATH must be set)
endif

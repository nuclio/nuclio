all: controller nuclio-build nuclio-deploy
	@echo Done.

nuclio-build: ensure-gopath
	go build -o ${GOPATH}/bin/nuclio-build cmd/nuclio-build/main.go

nuclio-deploy: ensure-gopath
	go build -o ${GOPATH}/bin/nuclio-deploy cmd/nuclio-deploy/main.go

controller:
	go build -o cmd/controller/_output/controller cmd/controller/main.go
	cd cmd/controller && docker build -t nuclio/controller .
	rm -rf cmd/controller/_output

.PHONY: ensure-gopath
check-gopath:
ifndef GOPATH
    $(error GOPATH must be set)
endif

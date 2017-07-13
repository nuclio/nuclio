cfg ?= config.yml

all:
	$(error please pick a target)


controller-docker:
	cp $(cfg) cmd/controller/config.yml
	go build -o cmd/controller/controller cmd/controller/main.go
	cd cmd/controller && docker build -t nuclio/controlle .
	rm cmd/controller/config.yml
	rm cmd/controller/controller

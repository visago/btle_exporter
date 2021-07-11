REVISION          := $(shell git rev-parse HEAD)
VERSION          := $(shell git describe --tags --always --dirty="-dev")
BRANCH          := $(shell git rev-parse --abbrev-ref HEAD)
DATE             := $(shell date -u '+%Y-%m-%dT%H:%M:%S+00:00')
VERSION_FLAGS    := -ldflags='-X "main.BuildVersion=$(VERSION)" -X "main.BuildRevision=$(REVISION)" -X "main.BuildTime=$(DATE)" -X "main.BuildBranch=$(BRANCH)"'
BINARY	:= bin/btle_exporter

all:	lint build

build:
	go build -o ${BINARY} ${VERSION_FLAGS} main.go

lint:
	gofmt -w main.go

run:
	go run ${VERSION_FLAGS} main.go
	
clean:
	rm -rf ${BINARY}
		
install:	build
	sudo cp -f bin/btle_exporter /usr/bin
	sudo cp btle_exporter.service  /etc/systemd/system/btle_exporter.service
	sudo systemctl daemon-reload
	sudo systemctl restart btle_exporter
	sudo systemctl status btle_exporter

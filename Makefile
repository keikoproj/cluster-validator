.PHONY: test docker clean all

COMMIT=`git rev-parse HEAD`
BUILD=`date +%FT%T%z`
LDFLAG_LOCATION=github.com/keikoproj/cluster-validator

LDFLAGS=-ldflags "-X ${LDFLAG_LOCATION}.buildDate=${BUILD} -X ${LDFLAG_LOCATION}.gitCommit=${COMMIT}"

GIT_TAG=$(shell git rev-parse --short HEAD)
IMAGE ?= cluster-validator:latest

build:
	CGO_ENABLED=0 go build ${LDFLAGS} -o bin/cluster-validator github.com/keikoproj/cluster-validator
	chmod +x bin/cluster-validator

test:
	go test -v ./... -coverprofile coverage.txt
	go tool cover -html=coverage.txt -o coverage.html

docker-build:
	docker build -t $(IMAGE) .

docker-push:
	docker push ${IMAGE}
# Include variables from the .env file
include .env
export PROJECT_DIR := $(CURDIR)

## help: print this help message
.PHONY: help
help:
	@echo "Usage:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## run flags=$1: run this programme with flags
.PHONY: run
run:
	@go run . ${flags}

## build: build the cmd/api application
.PHONY: build
build:
	@go build --ldflags='-w -s' . -o ./main

## build-test: build test executable
.PHONY: build-test
build-test:
	@go test -c --ldflags='-w -s' ./allocator -o ./browser.test

## run-test flags=$1: run test executable with flags
.PHONY: run-test
run-test:
	@./browser-test ${flags}
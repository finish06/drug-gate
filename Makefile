.PHONY: build test-unit test-coverage run vet lint

VERSION ?= dev

build:
	go build -ldflags="-X github.com/finish06/drug-gate/internal/version.Version=$(VERSION)" -o bin/server ./cmd/server

run:
	go run ./cmd/server

test-unit:
	go test ./... -short -count=1

test-coverage:
	go test ./... -coverprofile=coverage.out -count=1
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html

vet:
	go vet ./...

lint:
	golangci-lint run ./...

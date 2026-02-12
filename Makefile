.PHONY: build test lint

build:
	mkdir -p bin
	go build -o bin/skillpm ./cmd/skillpm

test:
	go test ./... -count=1

lint:
	test -z "$(gofmt -l .)"
	go vet ./...

.PHONY: build test test-sync-regression lint

build:
	mkdir -p bin
	go build -o bin/skillpm ./cmd/skillpm

test:
	go test ./... -count=1

test-sync-regression:
	go test ./cmd/skillpm -count=1 -run 'TestSync(OutputShowsChangedWithRiskOutcome|JSONOutputReflectsNoopState|CmdStrictFlagFailsOnRisk)|TestTotalSyncActions'

lint:
	test -z "$(gofmt -l .)"
	go vet ./...

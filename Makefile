.PHONY: build test test-integration lint image clean

BINARY := prow-agent-eval
IMAGE := prow-agent-eval

build:
	go build -o bin/$(BINARY) ./cmd/prow-agent-eval

test:
	go test ./... -v

test-integration:
	go test -tags=integration ./test/integration/... -v -count=1

lint:
	golangci-lint run ./...

image:
	docker build -t $(IMAGE) .

clean:
	rm -rf bin/

.PHONY: build test test-integration lint image clean vendor-harness test-python install-python

BINARY := prow-agent-eval
IMAGE := prow-agent-eval
HARNESS_TAG := v1.39.3
VENV := .venv

build:
	go build -o bin/$(BINARY) ./cmd/prow-agent-eval

test:
	go test ./... -v

test-integration:
	go test -tags=integration ./test/integration/... -v -count=1

vendor-harness:
	git clone --depth 1 --branch $(HARNESS_TAG) \
		https://github.com/opendatahub-io/agent-eval-harness.git vendor/agent-eval-harness \
		|| git -C vendor/agent-eval-harness fetch --depth 1 origin tag $(HARNESS_TAG) \
		&& git -C vendor/agent-eval-harness checkout $(HARNESS_TAG)

install-python:
	@test -d $(VENV) || python3 -m venv $(VENV)
	$(VENV)/bin/pip install -q -e ".[test]"

test-python:
	@test -d $(VENV) || python3 -m venv $(VENV)
	$(VENV)/bin/pip install -q -e ".[test]"
	AGENT_EVAL_HARNESS_ROOT=$$(pwd)/vendor/agent-eval-harness \
		$(VENV)/bin/pytest tests/ -q

lint:
	golangci-lint run ./...

image:
	docker build -t $(IMAGE) .

clean:
	rm -rf bin/ vendor/agent-eval-harness

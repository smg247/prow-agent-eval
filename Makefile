.PHONY: build test test-integration install image clean vendor-harness

IMAGE := prow-agent-eval
HARNESS_TAG := v1.39.3
VENV := .venv

build: install
	@echo "prow-agent-eval installed in $(VENV)"

install:
	@test -d $(VENV) || python3 -m venv $(VENV)
	$(VENV)/bin/pip install -q -e ".[test]"

test: install
	AGENT_EVAL_HARNESS_ROOT=$$(pwd)/vendor/agent-eval-harness \
		$(VENV)/bin/pytest tests/ -q

test-integration: install
	AGENT_EVAL_HARNESS_ROOT=$$(pwd)/vendor/agent-eval-harness \
		$(VENV)/bin/pytest tests/integration/ -q

vendor-harness:
	git clone --depth 1 --branch $(HARNESS_TAG) \
		https://github.com/opendatahub-io/agent-eval-harness.git vendor/agent-eval-harness \
		|| git -C vendor/agent-eval-harness fetch --depth 1 origin tag $(HARNESS_TAG) \
		&& git -C vendor/agent-eval-harness checkout $(HARNESS_TAG)

image:
	docker build -t $(IMAGE) .

clean:
	rm -rf $(VENV) vendor/agent-eval-harness

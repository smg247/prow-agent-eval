FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

ARG HARNESS_TAG=v1.39.3

RUN microdnf install -y python3.11 python3.11-pip git make && \
    microdnf clean all

RUN git clone --depth 1 --branch ${HARNESS_TAG} \
    https://github.com/opendatahub-io/agent-eval-harness.git /opt/agent-eval-harness

COPY . /opt/prow-agent-eval
WORKDIR /opt/prow-agent-eval

RUN pip3.11 install --no-cache-dir /opt/agent-eval-harness && \
    pip3.11 install --no-cache-dir .

ENV AGENT_EVAL_HARNESS_ROOT=/opt/agent-eval-harness

LABEL io.k8s.display-name="Prow Agent Eval" \
      io.openshift.tags="prow,eval,agent"

ENTRYPOINT ["prow-agent-eval"]

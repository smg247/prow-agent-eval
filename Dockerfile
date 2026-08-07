FROM registry.access.redhat.com/ubi9/ubi:latest AS builder

WORKDIR /go/src/prow-agent-eval

ENV PATH="/go/bin:${PATH}"
ENV GOPATH="/go"

COPY . .

RUN dnf install -y go make && \
    make build

FROM registry.access.redhat.com/ubi9/ubi:latest AS base

COPY --from=builder /go/src/prow-agent-eval/bin/prow-agent-eval /usr/local/bin/prow-agent-eval

RUN dnf install -y git make && \
    dnf clean all && \
    rm -rf /var/cache/dnf

LABEL io.k8s.display-name="Prow Agent Eval" \
      io.openshift.tags="prow,eval,agent"

ENTRYPOINT ["prow-agent-eval"]

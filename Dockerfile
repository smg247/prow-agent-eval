FROM golang:1.23 AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /prow-agent-eval ./cmd/prow-agent-eval

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates git make \
    && rm -rf /var/lib/apt/lists/*
COPY --from=builder /prow-agent-eval /usr/local/bin/prow-agent-eval
ENTRYPOINT ["prow-agent-eval"]

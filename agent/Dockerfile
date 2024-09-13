FROM golang:1.22.5 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY /. ./

RUN ls -la /app
ARG GOARCH
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 go build -v -o platform-operator-agent agent/main.go

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ENV LOG_LEVEL="info"

WORKDIR /

COPY --from=builder /app/platform-operator-agent /platform-operator-agent

EXPOSE 8080

USER 65532:65532

ENTRYPOINT ["/platform-operator-agent"]

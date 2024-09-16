FROM golang:1.22.7

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

COPY test/maploader/*.go ./
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /maploader
# syntax=docker/dockerfile:1
# check=error=true

# Latest version: https://hub.docker.com/_/golang/tags
FROM golang:1.26.2-trixie AS base

WORKDIR /src

RUN apt-get update \
    && apt-get install --assume-yes --no-install-recommends \
        ca-certificates \
        tree \
        git \
        openssh-client

FROM base AS builder-download

ARG GOARCH=amd64

COPY go.mod .
COPY go.sum .

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

FROM builder-download AS build

COPY . .

ARG GOOS=linux
ARG GOARCH=amd64
ARG GO_MODULE=github.com/specsnl/specs-cli
ARG SPECS_VERSION=dev

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go generate \
    && CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build \
        -trimpath \
        -tags netgo \
        -ldflags "-s -w -X ${GO_MODULE}/pkg/cmd.Version=${SPECS_VERSION}" -o ./specs

# Latest version: https://hub.docker.com/_/debian/tags
FROM debian:13.4-slim

COPY --from=build /src/specs /usr/local/bin

CMD ["specs"]

FROM scratch AS binary

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs
COPY --from=build /src/specs /
COPY --from=build /etc/passwd /etc/passwd

CMD ["/specs"]

FROM scratch AS export

COPY --from=build /src/specs /specs

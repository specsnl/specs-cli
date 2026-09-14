# syntax=docker/dockerfile:1
# check=error=true

# Latest version: https://hub.docker.com/_/golang/tags
FROM --platform=$BUILDPLATFORM golang:1.27.1-trixie AS base

WORKDIR /src

RUN apt-get update \
    && apt-get install --assume-yes --no-install-recommends \
        ca-certificates \
        tree \
        git \
        openssh-client

FROM base AS builder-download

COPY go.mod .
COPY go.sum .

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

FROM builder-download AS build

COPY . .

ARG TARGETOS
ARG TARGETARCH
ARG GOOS
ARG GOARCH
ARG GO_MODULE=github.com/specsnl/specs-cli
ARG SPECS_VERSION=dev

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go generate \
    && CGO_ENABLED=0 GOOS=${GOOS:-$TARGETOS} GOARCH=${GOARCH:-$TARGETARCH} go build \
        -trimpath \
        -tags netgo \
        -ldflags "-s -w -X ${GO_MODULE}/internal/cmd.Version=${SPECS_VERSION}" -o ./specs

FROM scratch AS export

COPY --from=build /src/specs /specs

# Latest version: https://github.com/charmbracelet/vhs/pkgs/container/vhs
FROM ghcr.io/charmbracelet/vhs:v0.11.0 AS vhs

ARG TARGETARCH

# Latest version: https://github.com/go-task/task/releases/latest
ARG TASK_VERSION=3.53.1
# Latest version: https://download.docker.com/linux/static/stable/
ARG DOCKER_VERSION=29.8.0
# Latest version: https://github.com/docker/compose/releases/latest
ARG COMPOSE_VERSION=5.5.1

RUN apt-get update \
    && apt-get install --assume-yes --no-install-recommends \
        ca-certificates \
        curl \
        git \
    && rm -rf /var/lib/apt/lists/*

RUN set -eux; \
    case "${TARGETARCH}" in \
        amd64) altarch=x86_64 ;; \
        arm64) altarch=aarch64 ;; \
        *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    curl --fail --silent --show-error --location \
        "https://github.com/go-task/task/releases/download/v${TASK_VERSION}/task_linux_${TARGETARCH}.tar.gz" \
        | tar --extract --gzip --directory /usr/bin task; \
    curl --fail --silent --show-error --location \
        "https://download.docker.com/linux/static/stable/${altarch}/docker-${DOCKER_VERSION}.tgz" \
        | tar --extract --gzip --directory /usr/bin --strip-components=1 docker/docker; \
    mkdir -p /usr/local/lib/docker/cli-plugins; \
    curl --fail --silent --show-error --location --output /usr/local/lib/docker/cli-plugins/docker-compose \
        "https://github.com/docker/compose/releases/download/v${COMPOSE_VERSION}/docker-compose-linux-${altarch}"; \
    chmod +x /usr/local/lib/docker/cli-plugins/docker-compose

RUN git config --system init.defaultBranch main

# Latest version: https://hub.docker.com/_/debian/tags
FROM debian:13.6-slim AS debian

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /src/specs /usr/local/bin/specs

RUN groupadd --gid 1000 specs \
    && useradd --uid 1000 --gid 1000 --create-home --shell /bin/bash specs \
    && mkdir -p /config /work \
    && chown specs:specs /config /work

# Pinned so the registry lands in /config whatever uid the container runs as;
# xdg would otherwise resolve it below $HOME.
ENV XDG_CONFIG_HOME=/config

WORKDIR /work
USER specs

ENTRYPOINT ["specs"]

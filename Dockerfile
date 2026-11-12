# syntax=docker/dockerfile:1
# check=error=true

# Latest version: https://github.com/go-task/task/releases/latest
ARG TASK_VERSION=3.53.1
# Latest version: https://download.docker.com/linux/static/stable/
ARG DOCKER_VERSION=29.8.0
# Latest version: https://github.com/docker/compose/releases/latest
ARG COMPOSE_VERSION=5.5.1

# Latest version: https://hub.docker.com/_/golang/tags
FROM --platform=$BUILDPLATFORM golang:1.27.1-trixie AS base

WORKDIR /src

RUN apt-get update \
    && apt-get install --assume-yes --no-install-recommends \
        ca-certificates \
        tree \
        git \
        openssh-client \
    && rm -rf /var/lib/apt/lists/*

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
FROM ghcr.io/charmbracelet/vhs:v0.12.0 AS vhs

ARG TARGETARCH
ARG TASK_VERSION
ARG DOCKER_VERSION
ARG COMPOSE_VERSION

# The downloads below are `curl | tar`, and the default /bin/sh reports only tar's exit status.
# Without pipefail a failed download is caught by tar choking on the stream, not by the shell.
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

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

# Latest version: https://hub.docker.com/r/bats/bats/tags
FROM bats/bats:1.14.0 AS bats

ARG TARGETARCH
ARG DOCKER_VERSION

# Latest version: https://github.com/bats-core/bats-support/releases/latest
ARG BATS_SUPPORT_VERSION=0.3.0
# Latest version: https://github.com/bats-core/bats-assert/releases/latest
ARG BATS_ASSERT_VERSION=2.2.4
# Latest version: https://github.com/bats-core/bats-file/releases/latest
ARG BATS_FILE_VERSION=0.4.0

# busybox ash, since this stage is Alpine and carries no bash.
SHELL ["/bin/ash", "-o", "pipefail", "-c"]

RUN apk add --no-cache \
    curl \
    tar

RUN set -eux; \
    case "${TARGETARCH}" in \
        amd64) altarch=x86_64 ;; \
        arm64) altarch=aarch64 ;; \
        *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    curl --fail --silent --show-error --location \
        "https://download.docker.com/linux/static/stable/${altarch}/docker-${DOCKER_VERSION}.tgz" \
        | tar --extract --gzip --directory /usr/bin --strip-components=1 docker/docker; \
    for spec in "support:${BATS_SUPPORT_VERSION}" "assert:${BATS_ASSERT_VERSION}" "file:${BATS_FILE_VERSION}"; do \
        name="bats-${spec%%:*}"; \
        mkdir -p "/usr/lib/bats/${name}"; \
        curl --fail --silent --show-error --location \
            "https://github.com/bats-core/${name}/archive/refs/tags/v${spec#*:}.tar.gz" \
            | tar --extract --gzip --directory "/usr/lib/bats/${name}" --strip-components=1; \
    done

ENV BATS_LIB_PATH=/usr/lib/bats

# Latest version: https://hub.docker.com/_/debian/tags
FROM debian:13.7-slim AS debian

ARG TARGETARCH
ARG TASK_VERSION
ARG DOCKER_VERSION
ARG COMPOSE_VERSION

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

COPY --from=build /src/specs /usr/local/bin/specs

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

# A container carries no user gitconfig, so a `git init` hook would pick git's
# built-in default and hand back a `master` branch where the same template run on
# a host produces `main`.
RUN git config --system init.defaultBranch main

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

# syntax=docker/dockerfile:1
# check=error=true

# Latest version: https://hub.docker.com/_/golang/tags
FROM golang:1.27.1-trixie AS base

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
        -ldflags "-s -w -X ${GO_MODULE}/internal/cmd.Version=${SPECS_VERSION}" -o ./specs

# Latest version: https://hub.docker.com/_/debian/tags
FROM debian:13.6-slim

COPY --from=build /src/specs /usr/local/bin

CMD ["specs"]

FROM scratch AS binary

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs
COPY --from=build /src/specs /
COPY --from=build /etc/passwd /etc/passwd

CMD ["/specs"]

FROM scratch AS export

COPY --from=build /src/specs /specs

# Recording image for the documentation GIFs (see docs/content/docs/architecture/demo.md).
# Upstream brings ttyd, ffmpeg and the fonts; everything added here exists because the
# recorded template runs it: specs-laravel-project declares `task setup:env-file:local`,
# `task md:fixstyle`, `git init` and `git add .` as post-use hooks, and md:fixstyle in turn
# shells out to `docker compose`.
# Latest version: https://github.com/charmbracelet/vhs/pkgs/container/vhs
FROM ghcr.io/charmbracelet/vhs:v0.11.0 AS vhs

# Provided by BuildKit: amd64 or arm64.
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

# The docker CLI and the compose plugin are clients only — they talk to the host daemon
# through the socket mounted by the vhs compose service.
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

# Without this, the `git init` hook spends nine lines of the frame warning about git's own
# default branch name — noise about git, not about specs.
RUN git config --system init.defaultBranch main

# /usr/local/bin is left empty on purpose: the compose service mounts ./dev over it so the
# freshly built Linux `specs` is the one on PATH inside the recording.

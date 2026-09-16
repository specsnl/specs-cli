# Specs CLI — Release Plan

## Overview

This document describes the release pipeline: how binaries are built and distributed,
how GitHub Releases are created, how the Homebrew formula is updated, how the container image is
published, and what CI workflows are needed.

---

## Tooling: GoReleaser

[GoReleaser](https://github.com/goreleaser/goreleaser) handles:

- Cross-compilation for all target platforms
- Archive packaging (`.tar.gz` for Unix, `.zip` for Windows)
- SHA-256 checksum file generation
- GitHub Release creation and asset uploads
- Homebrew tap cask updates, on two channels: `specs` (stable) and `specs@rc` (release candidates)

GoReleaser runs only in the release workflow — not needed for local development or CI tests.

---

## Version Injection

```text
-X github.com/specsnl/specs-cli/internal/cmd.Version=<version>
```

GoReleaser sets `Version` to the Git tag (e.g. `1.2.3`) automatically through `-ldflags`.

---

## Target Platforms

| OS       | Architecture |
|----------|--------------|
| `linux`  | `amd64`      |
| `linux`  | `arm64`      |
| `darwin` | `amd64`      |
| `darwin` | `arm64`      |

---

## GoReleaser Configuration (`.goreleaser.yml`)

### `builds`

```yaml
builds:
  - main: .
    binary: specs
    flags:
      - -trimpath
      - -tags=netgo
    ldflags:
      - -s -w
      - -X github.com/specsnl/specs-cli/internal/cmd.Version={{ .Version }}
    env:
      - CGO_ENABLED=0
    goos: [linux, darwin]
    goarch: [amd64, arm64]
```

### `archives`

```yaml
archives:
  - format: tar.gz
    name_template: "specs_{{ .Os }}_{{ .Arch }}"
```

### `checksum`

```yaml
checksum:
  name_template: "checksums.txt"
```

### `release`

```yaml
release:
  github:
    owner: specsnl
    name: specs-cli
  draft: false
  prerelease: auto
```

### `homebrew_casks`

Two casks are published from one definition, using a YAML anchor so the shared configuration cannot
drift between them:

```yaml
homebrew_casks:
  # The release-candidate channel, updated on every tag — prereleases included.
  # `binaries` is set explicitly because the cask is named specs@rc: without it
  # the installed binary would inherit the cask name rather than being `specs`.
  - &cask
    name: specs@rc
    binaries: [specs]
    repository:
      owner: specsnl
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    homepage: "https://github.com/specsnl/specs-cli"
    description: "General-purpose developer CLI"
    directory: Casks
    hooks:
      post:
        install: |
          if OS.mac?
            system_command "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "#{staged_path}/specs"]
          end

  # The stable channel: the same cask under the plain name, skipped whenever the
  # tag is a prerelease, so `brew install specsnl/tap/specs` never moves onto an
  # -rc build.
  - <<: *cask
    name: specs
    skip_upload: auto
```

### Release channels

| Tag           | `specs@rc` | `specs` |
|---------------|------------|---------|
| `v1.2.0`      | updated    | updated |
| `v1.2.0-rc.1` | updated    | skipped |

`skip_upload: auto` is what draws the line: GoReleaser skips that cask's upload when the tag carries
a prerelease identifier. The RC cask sets no `skip_upload`, so it follows every tag.

Two things are easy to get wrong here:

- **`binaries: [specs]` is not optional on the RC cask.** Left unset, the installed binary name is
  derived from the cask name, and `specs@rc` is not a command anyone wants to type. The generated
  `Casks/specs@rc.rb` must contain `binary "specs"`.
- **The explicit `name:` on the second entry must stay.** It overrides the anchor's `specs@rc`;
  YAML gives an explicit key precedence over a merged one, so the order within the mapping does not
  matter, but removing it would publish the same cask twice.

Verify both with a dry run before tagging — see [Local Dry Run](#local-dry-run).

---

## Homebrew Tap

A separate public repository is required: `github.com/specsnl/homebrew-tap`.

Structure after first release:

```text
homebrew-tap/
  Casks/
    specs.rb        ← generated and committed by GoReleaser on every stable release
    specs@rc.rb     ← generated and committed on every release, prereleases included
  README.md
```

Users install with:

```shell
brew tap specsnl/tap
brew install --cask specs        # stable
brew install --cask specs@rc     # release candidates
```

Both casks install a binary called `specs`, so Homebrew will refuse to link the second one while the
first is installed. Switching channels means uninstalling the other cask first. No `conflicts_with`
stanza is declared — the link failure is already clear about the cause.

### Setup Steps

1. Create the `specsnl/homebrew-tap` repository (public, with a `README.md`).
2. Create the `Casks/` directory with a `.gitkeep` placeholder.
3. Create a GitHub token with `contents: write` on that repository and store it as
   `HOMEBREW_TAP_GITHUB_TOKEN` in the `specs-cli` repo secrets.
4. Add the `homebrew_casks` section to `.goreleaser.yml` (shown above).

---

## Container Image

GoReleaser does not build the image. The `image` and `image-manifest` jobs in `release.yml` do, on
the same `v*` tags, by calling the org's shared workflows:

- [`build-go-cli.yml`](https://github.com/specsnl/github-actions/blob/main/.github/workflows/build-go-cli.yml)
- [`merge-go-cli.yml`](https://github.com/specsnl/github-actions/blob/main/.github/workflows/merge-go-cli.yml)

Both are documented in
[specsnl/github-actions](https://github.com/specsnl/github-actions/blob/main/docs/go-cli.md). What
stays in this repository is the Dockerfile and the three things only this repository can know: which
stage to publish, what the version build arg is called, and what a working image has to be able to
do.

| What          | Value                                               |
|---------------|-----------------------------------------------------|
| Registry      | `ghcr.io/specsnl/specs-cli`                         |
| Built from    | the Dockerfile's `debian` stage                     |
| Base          | `debian:13.6-slim`                                  |
| Platforms     | `linux/amd64`, `linux/arm64`                        |
| Runs as       | `specs`, uid/gid `1000`                             |
| Entrypoint    | `specs`                                             |
| Authenticates | the built-in `GITHUB_TOKEN`, with `packages: write` |
| Published by  | `specsnl/github-actions@2.4.0`                      |

### Dockerfile stages

Only one of them ships:

| Stage              | Used by                                | Published |
|--------------------|----------------------------------------|-----------|
| `base`             | every other stage                      | no        |
| `builder-download` | `task test` (the `go-builder` service) | no        |
| `build`            | compiles the binary                    | no        |
| `export`           | `task build` copies the binary out     | no        |
| `vhs`              | `task demo:record:*`                   | no        |
| `debian`           | the release job                        | **yes**   |

`debian` is deliberately the last stage in the file, so a bare `docker build .` produces the image
that ships rather than a development tool. The workflows still pass `--target debian` explicitly —
being last is a safe default, being named is the contract.

The runtime borrows two things from the builder: the binary, and `ca-certificates.crt`. The slim
Debian variants ship no CA bundle, and without one every HTTPS template source fails to clone.
Nothing else is needed — `internal/util/git` clones through go-git, so there is no `git` binary in
the image. Debian rather than scratch or distroless is a hooks decision: `internal/hooks` executes
them through `bash -c` and refuses to run when bash is absent, so a shell-less image would silently
drop every template that defines one.

### One runner per architecture

Nothing is emulated. The `image` job is a matrix over the two release platforms, each on a runner of
that architecture:

| Platform      | Runner             |
|---------------|--------------------|
| `linux/amd64` | `ubuntu-24.04`     |
| `linux/arm64` | `ubuntu-24.04-arm` |

Arm runners are free for public repositories, which this one is.

QEMU would be the alternative, and it is the wrong one here. The runtime stage creates its user with
a `RUN`, and a `RUN` always executes on the target platform — so an emulated arm64 build runs
`useradd` under QEMU, slowly, and gives up the ability to execute the image it just produced. A
native runner builds at full speed and can run its own output, which is what lets the pull-request
job smoke-test both architectures rather than only the one it happens to be on.

The consequence is that each job produces a single-platform image and the multi-arch manifest has to
be assembled afterwards — each build pushes untagged by digest, and `image-manifest` joins the
digests into one tagged list. That whole dance lives in the shared workflows; see
[How the image pipeline works](https://github.com/specsnl/github-actions/blob/main/docs/pipeline.md).

### Cross-compilation

The `base` stage is pinned to `--platform=$BUILDPLATFORM`, so the Go toolchain always runs natively
on the builder and cross-compiles from there. On a native runner the two platforms coincide and this
changes nothing; it is what keeps a local `docker buildx build --platform linux/amd64,linux/arm64`
from emulating the compiler, and `CGO_ENABLED=0` means there is nothing lost by it.

The `build` stage resolves its target as `GOOS=${GOOS:-$TARGETOS}` / `GOARCH=${GOARCH:-$TARGETARCH}`.
`GOOS` and `GOARCH` carry no defaults on purpose — the image build leaves them unset and follows
the platform buildx asked for, while `task build` sets both explicitly through `docker buildx bake`
to get a host binary out of the `export` stage.

### Image tags

`merge-go-cli.yml` owns the tag set. For this repository it produces:

| Tag pushed    | Image tags produced           |
|---------------|-------------------------------|
| `v1.2.3`      | `1.2.3`, `1.2`, `1`, `latest` |
| `v0.4.1`      | `0.4.1`, `0.4`, `latest`      |
| `v1.2.0-rc.1` | `1.2.0-rc.1`                  |

A prerelease publishes its own version and moves nothing anyone could be following — the same line
the `specs` and `specs@rc` casks draw. The `{{major}}` tag is also withheld while the project is on
`v0`, where a major number carries no compatibility promise.

Neither job passes a version. `build-go-cli.yml` defaults it to the tag **without its leading
`v`** — the same string GoReleaser injects, so the image and the Homebrew binary cut from one tag
never disagree about what they are. It reaches the binary as the `SPECS_VERSION` build arg, named
by `version-build-arg`, which is the one input here with no default: it is named after the binary
and differs per repository, and a wrong value fails silently, shipping an image that reports `dev`.
The pull-request guard asserts against exactly that.

### One-time setup

The GHCR package is created by the first push and is **private** until someone changes it. After
the first release, open the package settings and either make it public or link it to the repository.
Nothing in the workflow can do this.

### Pre-release guard

`ci.yml` builds `--target debian` on every pull request, on the same matrix of native runners and
without pushing, then runs `test/image.bats` against each. Both architectures are executed, not
merely built.

It calls the `build-image` action directly rather than `build-go-cli.yml`, because the image has to
be built and run in the same job: a reusable workflow would load it into a daemon this job cannot
reach. `load: true` builds one platform into the runner's local daemon and reports the reference as
its `image` output, which the suite takes as `IMAGE`.

The same suite backs `task image:smoke`, so a green local run and a green pull request check the
same things:

```shell
task image:build   # build the runtime image as specs-cli:dev
task image:smoke   # build it, then run the acceptance checks
```

It asserts that `--version` reports the injected version rather than `dev`, that the default user is
`uid=1000(specs)`, that bash is on PATH, that a template scaffolds into a bind mount with its hooks
run and its files owned by the invoking user, and that a run without a terminal refuses instead of
quietly falling back to defaults. Each is its own test, so one broken thing does not hide the next.

### Where bats comes from

The suite needs bats, three of its libraries, and a docker client — and the two environments get
them differently, because only one of them has docker to begin with.

|       | bats                                                                                                                   | docker                                               |
|-------|------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------|
| CI    | `bats-core/bats-action`, which also installs `bats-support`, `bats-assert` and `bats-file` and exports `BATS_LIB_PATH` | native on the runner                                 |
| Local | the Dockerfile's `bats` stage — `bats/bats` plus the same three libraries and a static docker client                   | `docker-socket-proxy`, shared with the `vhs` service |

Running the tests in a container while they drive the host's daemon has one consequence worth
knowing before editing them: **every path the tests bind-mount has to exist on the host**, because
that is where the daemon resolves it. The compose service handles this by mounting the repository at
its own host path (`${PWD}:${PWD}`) and pointing `TMPDIR` at `/tmp/specs-image-smoke`, mounted at
the same path on both sides — the trick `vhs` already uses for `/tmp/specs-demo`. A test that writes
to a container-only path and mounts it will fail in a way that looks like a missing file.

---

## Tagging and Versioning

- Tags follow [Semantic Versioning](https://semver.org/): `vMAJOR.MINOR.PATCH`
- Pre-release suffixes are supported: `v1.0.0-rc.1`, `v1.0.0-beta.1`
- GoReleaser strips the leading `v` when injecting into `-ldflags`.
- A prerelease tag reaches only the `specs@rc` cask; the stable `specs` cask is left on the last
  stable version. See [Release channels](#release-channels).

---

## GitHub Actions Workflows

### CI workflow — `ci.yml`

**Trigger:** push and pull_request on any branch.

| Step          | Command                                                                              |
|---------------|--------------------------------------------------------------------------------------|
| Checkout      | `actions/checkout` with `fetch-depth: 0`                                             |
| Setup Go      | `actions/setup-go` pinned to `go.mod` version                                        |
| Cache modules | `actions/cache` on Go module and build caches                                        |
| Vet           | `go vet ./...`                                                                       |
| Test          | `go test -race -count=1 ./...`                                                       |
| Build (smoke) | `go build -o /dev/null .`                                                            |
| Image         | `build-image` with `load` on a native runner per architecture, then the smoke script |

### Release workflow — `release.yml`

**Trigger:** push of a tag matching `v*`.

| Job              | Detail                                                                   |
|------------------|--------------------------------------------------------------------------|
| `release`        | Checkout with `fetch-depth: 0`, set up Go, cache modules, run GoReleaser |
| `version`        | Strips the leading `v` off the tag for the other two                     |
| `image`          | Matrix over amd64 and arm64, calling `build-go-cli.yml`                  |
| `image-manifest` | Calls `merge-go-cli.yml` to join the digests into the tagged manifest    |

`release` is independent of the rest; `image` waits on `version`, and `image-manifest` on both.

Required secrets:

| Secret                      | Purpose                                                        |
|-----------------------------|----------------------------------------------------------------|
| `GITHUB_TOKEN`              | Built-in; creates the GitHub Release and authenticates to GHCR |
| `HOMEBREW_TAP_GITHUB_TOKEN` | PAT with `contents: write` on `specsnl/homebrew-tap`           |

Permissions are granted per job rather than to the whole workflow: `release` takes
`contents: write`, while `image` and `image-manifest` take `contents: read` and `packages: write`.
Neither side needs what the other has.

---

## Local Dry Run

```shell
task release:dry-run
```

Runs GoReleaser via Docker Compose (`--snapshot --clean`) without publishing.
Binaries and archives land in `dist/` for inspection.

---

## Release Checklist

1. All CI checks pass on `main`.
2. Release notes are ready.
3. `go.mod` / `go.sum` are committed and `go mod tidy` has been run.
4. Tag and push:

   ```shell
   git tag v1.0.0
   git push origin v1.0.0
   ```

5. Verify the GitHub Release was created.
6. Verify the Homebrew cask was updated in `specsnl/homebrew-tap`.
7. Test the Homebrew install on a clean machine:

   ```shell
   brew update && brew upgrade --cask specs
   ```

8. Verify the image was published, is multi-arch, and knows its own version:

   ```shell
   docker manifest inspect ghcr.io/specsnl/specs-cli:1.0.0   # amd64 + arm64
   docker run --rm ghcr.io/specsnl/specs-cli:1.0.0 --version # 1.0.0, not dev
   ```

   On the first release only, make the GHCR package public — see
   [One-time setup](#one-time-setup).

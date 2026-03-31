# Specs CLI — Release Plan

## Overview

This document describes the release pipeline for `specs`: how binaries are built and
distributed, how GitHub Releases are created, how the Homebrew formula is updated, and
what CI workflows are needed.

---

## Tooling: GoReleaser

[GoReleaser](https://github.com/goreleaser/goreleaser) is the single tool that handles:

- Cross-compilation for all target platforms
- Archive packaging (`.tar.gz` for Unix, `.zip` for Windows)
- SHA-256 checksum file generation
- GitHub Release creation and asset uploads
- Homebrew tap formula update

GoReleaser is run only in the release workflow (see below). It is **not** needed for local
development or the CI test workflow.

---

## Version injection

The existing mechanism in `Dockerfile` and `pkg/cmd/version.go` already accepts a version
string via `-ldflags`:

```
-X github.com/specsnl/specs-cli/pkg/cmd.Version=<version>
```

GoReleaser sets `Version` to the Git tag (e.g. `1.2.3`) automatically through this same
`-ldflags` mechanism. No source changes are needed.

---

## Target platforms

| OS | Architecture | Notes |
|----|-------------|-------|
| `linux` | `amd64` | Primary server target |
| `linux` | `arm64` | Raspberry Pi / ARM servers |
| `darwin` | `amd64` | macOS Intel |
| `darwin` | `arm64` | macOS Apple Silicon |
| `windows` | `amd64` | Windows 64-bit |

---

## GoReleaser configuration (`.goreleaser.yml`)

Place at the repository root. Key sections:

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
      - -X github.com/specsnl/specs-cli/pkg/cmd.Version={{ .Version }}
    env:
      - CGO_ENABLED=0
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    ignore:
      - goos: windows
        goarch: arm64
```

### `archives`

```yaml
archives:
  - format: tar.gz
    format_overrides:
      - goos: windows
        format: zip
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
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
  prerelease: auto   # tags like v1.0.0-rc1 become pre-releases automatically
```

### `brews` (Homebrew tap)

GoReleaser can push a formula update to a separate tap repository after each release.
See the [Homebrew tap](#homebrew-tap) section for the tap repo setup.

```yaml
brews:
  - name: specs
    repository:
      owner: specsnl
      name: homebrew-tap        # must exist before first release
    homepage: "https://github.com/specsnl/specs-cli"
    description: "General-purpose developer CLI"
    license: "MIT"
    folder: Formula
    install: |
      bin.install "specs"
    test: |
      system "#{bin}/specs", "version", "--dont-prettify"
```

---

## Homebrew tap

A **separate** public GitHub repository is required:

```
github.com/specsnl/homebrew-tap
```

This is a shared tap for the whole `specsnl` org — future tools can be added here without
users having to run `brew tap` again.

Structure after first release:

```
homebrew-tap/
  Formula/
    specs.rb     ← generated and committed by GoReleaser on every release
  README.md
```

Users install with:

```shell
brew tap specsnl/tap
brew install specs
```

`brew tap specsnl/tap` is shorthand for `specsnl/homebrew-tap`.

### Setup steps

1. Create the `specsnl/homebrew-tap` repository (public, with a `README.md`).
2. Create the `Formula/` directory with an empty placeholder or initial `specs.rb`.
3. Create a GitHub token (fine-grained or classic PAT) with `contents: write` on that
   repository and store it as the secret `HOMEBREW_TAP_GITHUB_TOKEN` in `specs-cli`.
4. Add the `brews` section to `.goreleaser.yml` (shown above).

GoReleaser commits directly to the tap repository as part of the release workflow.

---

## Tagging and versioning convention

- Tags follow [Semantic Versioning](https://semver.org/): `vMAJOR.MINOR.PATCH`
- Pre-release suffixes are supported: `v1.0.0-rc.1`, `v1.0.0-beta.1`
- GoReleaser strips the leading `v` when injecting into `-ldflags`, so `specs version`
  will print `1.2.3` (not `v1.2.3`). Adjust if desired.

---

## GitHub Actions workflows

### 1. CI workflow — `ci.yml`

**Trigger:** push and pull_request on any branch.

**Responsibility:** fast feedback — lint, vet, test, and a dry-run build on the host
(not through Docker/Taskfile) so CI is self-contained.

```
.github/workflows/ci.yml
```

Steps:

| Step | Command |
|------|---------|
| Checkout | `actions/checkout` with `fetch-depth: 0` (GoReleaser needs full history) |
| Setup Go | `actions/setup-go` pinned to the version in `go.mod` |
| Cache modules | `actions/cache` on `~/.cache/go/pkg/mod` and `~/.cache/go/build` |
| Vet | `go vet ./...` |
| Test | `go test -race -count=1 ./...` |
| Build (smoke) | `go build -o /dev/null .` |

Run on `ubuntu-latest`. No Docker involved in CI — Go toolchain is installed directly by
the action, which is simpler and faster.

### 2. Release workflow — `release.yml`

**Trigger:** push of a tag matching `v*` (e.g. `v1.0.0`).

**Responsibility:** produce and publish the release.

```
.github/workflows/release.yml
```

Steps:

| Step | Detail |
|------|--------|
| Checkout | `fetch-depth: 0` — GoReleaser needs all tags and commits |
| Setup Go | same version as `go.mod` |
| Cache modules | same as CI |
| Run GoReleaser | `goreleaser/goreleaser-action` with `distribution: goreleaser`, `version: v2.15.1` |

Required secrets:

| Secret | Purpose |
|--------|---------|
| `GITHUB_TOKEN` | Built-in; used by GoReleaser to create GitHub Release |
| `HOMEBREW_TAP_GITHUB_TOKEN` | Fine-grained PAT with `contents: write` on `specsnl/homebrew-tap` |

`GITHUB_TOKEN` requires `contents: write` permission at the workflow level:

```yaml
permissions:
  contents: write
```

---

## Local dry run

Before pushing a tag, verify that GoReleaser's config is correct by running a local build
that skips publishing entirely. This catches misconfigured `builds`, bad `ldflags`, and
archive naming issues without creating a GitHub Release or touching the Homebrew tap.

```shell
task release:dry-run
```

This delegates to the `goreleaser` Docker Compose service using the official
`goreleaser/goreleaser` image — the same image the GitHub Actions release workflow uses,
so local and CI behaviour match exactly. No local GoReleaser install is needed.

`--snapshot` forces a non-tagged build so no Git tag is required. `--clean` removes any
previous `dist/` output first. All binaries and archives land in `dist/` for inspection
but nothing is published.

### Compose service (`compose.yml`)

Add a `goreleaser` service alongside the existing `go-builder`:

```yaml
goreleaser:
  profiles: ["build"]
  # Latest version: https://hub.docker.com/r/goreleaser/goreleaser/tags
  image: goreleaser/goreleaser:v2.15.1
  working_dir: /src
  volumes:
    - .:/src
    - go-mod:/go/pkg/mod
    - go-build-cache:/root/.cache/go-build
```

### Taskfile task (`Taskfile.dist.yml`)

```yaml
release:dry-run:
  desc: Build a snapshot release locally without publishing
  cmds:
    - task: dc:run:goreleaser
      vars:
        SUB_CMD: release --snapshot --clean
```

---

## Release checklist

Before tagging a release:

1. All CI checks pass on `main`.
2. `CHANGELOG` (or release notes) are ready.
3. `go.mod` / `go.sum` are committed and `go mod tidy` has been run.
4. Create and push the tag:

   ```shell
   git tag v1.0.0
   git push origin v1.0.0
   ```

5. The release workflow triggers automatically and creates the GitHub Release.
6. Verify the Homebrew formula was updated in `specsnl/homebrew-tap`.
7. Test the Homebrew install on a clean machine:

   ```shell
   brew update && brew upgrade specs
   ```

---

## Directory layout after implementation

```
.github/
  workflows/
    ci.yml         ← test + vet + smoke build
    release.yml    ← GoReleaser release on tag push
.goreleaser.yml    ← GoReleaser configuration
```

Changes required to existing files:

- **`compose.yml`** — add the `goreleaser` service (see [Local dry run](#local-dry-run))
- **`Taskfile.dist.yml`** — add the `release:dry-run` task (see [Local dry run](#local-dry-run))

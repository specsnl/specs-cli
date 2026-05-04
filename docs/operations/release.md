# Specs CLI — Release Plan

## Overview

This document describes the release pipeline: how binaries are built and distributed,
how GitHub Releases are created, how the Homebrew formula is updated, and what CI workflows
are needed.

---

## Tooling: GoReleaser

[GoReleaser](https://github.com/goreleaser/goreleaser) handles:

- Cross-compilation for all target platforms
- Archive packaging (`.tar.gz` for Unix, `.zip` for Windows)
- SHA-256 checksum file generation
- GitHub Release creation and asset uploads
- Homebrew tap cask update

GoReleaser runs only in the release workflow — not needed for local development or CI tests.

---

## Version Injection

```
-X github.com/specsnl/specs-cli/pkg/cmd.Version=<version>
```

GoReleaser sets `Version` to the Git tag (e.g. `1.2.3`) automatically through `-ldflags`.

---

## Target Platforms

| OS | Architecture |
|----|-------------|
| `linux` | `amd64` |
| `linux` | `arm64` |
| `darwin` | `amd64` |
| `darwin` | `arm64` |

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
      - -X github.com/specsnl/specs-cli/pkg/cmd.Version={{ .Version }}
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

```yaml
homebrew_casks:
  - name: specs
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
```

---

## Homebrew Tap

A separate public repository is required: `github.com/specsnl/homebrew-tap`.

Structure after first release:

```
homebrew-tap/
  Casks/
    specs.rb     ← generated and committed by GoReleaser on every release
  README.md
```

Users install with:

```shell
brew tap specsnl/tap
brew install --cask specs
```

### Setup Steps

1. Create the `specsnl/homebrew-tap` repository (public, with a `README.md`).
2. Create the `Casks/` directory with a `.gitkeep` placeholder.
3. Create a GitHub token with `contents: write` on that repository and store it as
   `HOMEBREW_TAP_GITHUB_TOKEN` in the `specs-cli` repo secrets.
4. Add the `homebrew_casks` section to `.goreleaser.yml` (shown above).

---

## Tagging and Versioning

- Tags follow [Semantic Versioning](https://semver.org/): `vMAJOR.MINOR.PATCH`
- Pre-release suffixes are supported: `v1.0.0-rc.1`, `v1.0.0-beta.1`
- GoReleaser strips the leading `v` when injecting into `-ldflags`.

---

## GitHub Actions Workflows

### CI workflow — `ci.yml`

**Trigger:** push and pull_request on any branch.

| Step | Command |
|------|---------|
| Checkout | `actions/checkout` with `fetch-depth: 0` |
| Setup Go | `actions/setup-go` pinned to `go.mod` version |
| Cache modules | `actions/cache` on Go module and build caches |
| Vet | `go vet ./...` |
| Test | `go test -race -count=1 ./...` |
| Build (smoke) | `go build -o /dev/null .` |

### Release workflow — `release.yml`

**Trigger:** push of a tag matching `v*`.

| Step | Detail |
|------|--------|
| Checkout | `fetch-depth: 0` — GoReleaser needs all tags and commits |
| Setup Go | same version as `go.mod` |
| Cache modules | same as CI |
| Run GoReleaser | `goreleaser/goreleaser-action` |

Required secrets:

| Secret | Purpose |
|--------|---------|
| `GITHUB_TOKEN` | Built-in; used by GoReleaser to create GitHub Release |
| `HOMEBREW_TAP_GITHUB_TOKEN` | PAT with `contents: write` on `specsnl/homebrew-tap` |

The release workflow needs `permissions: contents: write`.

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

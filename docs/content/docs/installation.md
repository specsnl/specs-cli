---
title: Installation
weight: 1
---

## Homebrew (macOS)

```sh
brew install specsnl/tap/specs
```

### Release candidates

Two casks are published from the same tap, and they differ only in which releases they follow:

| Cask       | Follows                         | Install                             |
|------------|---------------------------------|-------------------------------------|
| `specs`    | Stable releases only            | `brew install specsnl/tap/specs`    |
| `specs@rc` | Every tag, prereleases included | `brew install specsnl/tap/specs@rc` |

Use `specs@rc` to try a release candidate — `v1.2.0-rc.1` and the like — before it is promoted. The
stable cask never moves onto a prerelease, so a `brew upgrade` on `specs` cannot pull one in by
accident.

Both casks install a binary called `specs`, so only one can be active at a time:

```sh
brew uninstall specs && brew install specsnl/tap/specs@rc   # switch to the RC channel
brew uninstall specs@rc && brew install specsnl/tap/specs   # switch back
```

## From source

```sh
go install github.com/specsnl/specs-cli@latest
```

## Download a binary

Download a pre-built binary from the [releases page](https://github.com/specsnl/specs-cli/releases).

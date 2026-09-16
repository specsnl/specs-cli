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

## Docker

An official image is published to GHCR on every release, for `linux/amd64` and `linux/arm64`:

```sh
docker run --rm -it -v "$PWD:/work" ghcr.io/specsnl/specs-cli use specsnl/my-template ./my-project
```

The entrypoint is `specs` itself, so everything after the image name is the command line you would
type locally.

### Tags

| Tag          | Moves to                                       |
|--------------|------------------------------------------------|
| `1.2.3`      | that exact release                             |
| `1.2`        | the newest patch of that minor                 |
| `1`          | the newest minor of that major — from `v1.0.0` |
| `latest`     | the newest stable release                      |
| `1.2.0-rc.1` | that exact prerelease                          |

A prerelease publishes only its own version tag: `latest` and the moving `1.2` / `1` tags never
point at an `-rc` build, the same line the `specs` and `specs@rc` casks draw. Pin to an exact
version in CI.

### Writing into a bind mount

`specs use` scaffolds onto the host filesystem, so the container's uid has to be able to write
there. The image runs as a non-root `specs` user at `1000:1000`, which is the first user on a
typical single-user Linux host — there, the command above just works.

Anywhere else, pass your own ids:

```sh
docker run --rm -it \
  --user "$(id -u):$(id -g)" \
  --env HOME=/tmp \
  --volume "$PWD:/work" \
  ghcr.io/specsnl/specs-cli use specsnl/my-template ./my-project
```

`HOME` belongs with `--user` rather than being optional decoration. An overridden uid owns no home
directory inside the image — Docker points `HOME` at `/home/specs` when the uid happens to be 1000
and at `/` otherwise, and neither is writable by a foreign uid. Anything that reads or writes below
the home directory, SSH keys in particular, needs somewhere real to go.

The template registry is unaffected by this: it hangs off `XDG_CONFIG_HOME`, which the image pins
to `/config`, not off `$HOME`.

### Prompts need a terminal

`-it` is what makes the interactive flow work. `specs` only prompts when its stdin is a terminal,
and a plain `docker run` gives it a pipe:

```sh
docker run --rm -it -v "$PWD:/work" ghcr.io/specsnl/specs-cli use specsnl/my-template ./my-project
```

Without a terminal there is no quiet fallback to defaults — a value nobody can supply is an error
that names itself, listing the keys it could not ask for. In a pipeline, answer up front instead:

```sh
docker run --rm \
  --user "$(id -u):$(id -g)" --env HOME=/tmp --volume "$PWD:/work" \
  ghcr.io/specsnl/specs-cli use specsnl/my-template ./my-project \
  --use-defaults --arg ProjectName=my-app --yes
```

`--use-defaults` takes the schema's defaults, `--arg K=V` and `--values file.json` supply values
explicitly, and the three combine.

### Hooks run other people's code

{{< callout type="warning" >}}
A template's hooks are shell commands from whoever wrote the template, and they run on the tree
`specs` just scaffolded. Reading them before running an unfamiliar template matters more here than
it does locally, because a container makes it that much easier to run one you have never seen.
{{< /callout >}}

Hooks are why the image is Debian rather than something minimal. They execute through `bash`, and a
hook is only as good as the commands it can reach, so the image ships the ones templates actually
use:

| Tool                 | Why it is there                                                              |
|----------------------|------------------------------------------------------------------------------|
| `bash`               | Every hook runs through it                                                   |
| `git`                | `git init` / `git add` — the most common closing hook                        |
| `task`               | Generated projects that use [Task](https://taskfile.dev) as their entrypoint |
| `docker` + `compose` | Hooks that drive a compose stack to do their work                            |

`git` is configured with `init.defaultBranch main`, so a `git init` hook produces the same branch
name it would on a host rather than falling back to git's built-in `master`.

For a **remote** template that defines hooks, `specs` prints the commands and asks before running
them. Without a terminal it cannot ask, so it warns and skips the hooks — a scaffold that looks
successful but is missing whatever the hooks were supposed to do. `--yes` is the way to run them
unattended, and it is the flag to reach for in CI. The confirmation only guards remote templates;
hooks in a local path template run without it.

To keep them from running at all:

| Flag          | Effect                                                 |
|---------------|--------------------------------------------------------|
| `--no-hooks`  | Skip the hooks, render everything else                 |
| `--safe-mode` | Also disable the env and filesystem template functions |

### Hooks that start containers

A hook that runs `docker` talks to the host daemon, so whatever it starts is a *sibling* container,
not a child. That needs two additions — the socket, and an identical path on both sides:

```sh
docker run --rm -it \
  --user "$(id -u):$(id -g)" --group-add 0 \
  --env HOME=/tmp \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  --volume "$PWD:$PWD" --workdir "$PWD" \
  ghcr.io/specsnl/specs-cli use specsnl/my-template ./my-project
```

`--group-add 0` is what gets a foreign uid past the socket's `root:root 0660`.

The path matters because the daemon resolves a sibling's bind mounts against the *host* filesystem.
Mounted at `/work`, a hook asks the daemon for `/work/...`, which on the host is some other
directory or none at all — the sibling starts empty and the hook fails against a tree that looks
perfectly fine from inside. Mounting `$PWD` at `$PWD` makes the two agree.

Templates whose hooks never invoke `docker` need none of this and can keep using `/work`.

### Keeping registered templates

`specs template download` and `specs template save` write to the registry under `/config`, which
disappears with the container unless it is given a volume:

```sh
docker run --rm -it \
  --volume specs-config:/config \
  --volume "$PWD:/work" \
  ghcr.io/specsnl/specs-cli template list
```

The one-shot `specs use <source> <target>` path needs none of this — it fetches, renders and
discards, which is what most container use looks like.

### Private templates over SSH

Cloning needs no `git` binary, but an SSH source does need credentials and a `known_hosts` file —
host key verification is not optional, and a missing `known_hosts` fails the clone before
authentication is even attempted. Mount the agent socket and your known hosts under whatever `HOME`
you set:

```sh
docker run --rm -it \
  --user "$(id -u):$(id -g)" \
  --env HOME=/tmp \
  --env SSH_AUTH_SOCK=/ssh-agent \
  --volume "$SSH_AUTH_SOCK:/ssh-agent" \
  --volume "$HOME/.ssh/known_hosts:/tmp/.ssh/known_hosts:ro" \
  --volume "$PWD:/work" \
  ghcr.io/specsnl/specs-cli use git@github.com:me/private-template ./my-project
```

With no agent running, mount a key at `/tmp/.ssh/id_ed25519` (or `id_rsa`, `id_ecdsa`) instead —
the same three names `specs` looks for on a host. Note that the agent socket is a macOS sore point:
Docker Desktop does not forward the host's `SSH_AUTH_SOCK`, so use
`--volume /run/host-services/ssh-auth.sock:/ssh-agent` there.

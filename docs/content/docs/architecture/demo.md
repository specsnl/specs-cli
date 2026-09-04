---
title: Demo Recordings
weight: 6
---

The documentation GIFs are recorded with [VHS](https://github.com/charmbracelet/vhs) from tapes
that live in the repository. A tape is a plain-text script of terminal settings and typed
commands, so everything a frame depends on — terminal size, font, shell, the `specs` build —
is pinned in-repo, and re-recording is one command rather than a set-up.

## The GIFs are snapshots, not tests

Nothing in CI records or compares them. There is no golden-file mechanism here and no
assertion that a GIF still matches the CLI.

**Recording always dirties the working tree.** The same tape never produces identical bytes
twice — frame timing, container ids and relative timestamps all differ between runs — so a
regenerated GIF that shows nothing new is pure churn in the diff and in the repository's
history. Re-record deliberately: when the recorded output actually changed, not as a reflex
after touching unrelated code.

## Re-recording

```sh
task demo:record:use
task demo:record:template
```

One tape per invocation, for the churn reason above — there is deliberately no
record-everything target. Naming a tape that does not exist fails on a precondition with the
list of tapes that do:

```console
$ task demo:record:nope
task: No such tape: docs/demo/nope.tape (available: template use)
```

Requirements on the host are **Docker and [Task](https://taskfile.dev), nothing else** — no
`vhs` binary, no font installation, no GitHub token. The templates are cloned over anonymous
HTTPS by `specs`' own [go-git](https://github.com/go-git/go-git) layer, so there is no secret
handling in this setup at all. Network access is required.

The first recording on a machine is slow: the recorded template's hooks pull roughly a
gigabyte of images. Later recordings reuse them and take seconds.

## What runs where

| Piece                                           | Role                                                                        |
|-------------------------------------------------|-----------------------------------------------------------------------------|
| `vhs` target in `Dockerfile`                    | Pinned `ghcr.io/charmbracelet/vhs` plus `git`, `task` and the docker client |
| `vhs` service in `compose.yml` (`demo` profile) | Mounts, the Docker socket, `HOME`, the host uid                             |
| `demo:binary` in `taskfiles/Taskfile.demo.yml`  | Builds a **Linux** `specs` into `dev/`                                      |
| `demo:record:<name>`                            | Precondition check, then runs the tape in the container                     |
| `docs/demo/*.tape`                              | The scripts                                                                 |
| `docs/static/demo/*.gif`                        | The committed output, served at `/demo/<name>.gif`                          |

### The Linux binary in `dev/`

The tape runs inside the container, so the host binary `task build` leaves at `./specs` is not
the one that can be recorded. `demo:binary` runs the same `docker buildx bake` invocation with
`GOOS=linux` and `--set go-binary.output=type=local,dest=dev`. `dev/` is gitignored and stays
deliberately separate from `./specs`; the compose service mounts it over the (empty)
`/usr/local/bin` in the image, which is what puts the recorded binary on `PATH`.

### The Docker socket, and why the scratch directory is outside the checkout

`specs-laravel-project` declares four `post-use` hooks:

```text
task setup:env-file:local
task md:fixstyle
git init
git add .
```

Recording a `specs use` that skips them would skip the single most interesting thing the
command does, so the image carries `git` and `task`, and `task md:fixstyle` in the generated
project routes through `docker compose` — which means `task` on `PATH` is not enough. The
`vhs` service mounts `/var/run/docker.sock` and the image carries a docker client and the
compose plugin, so the hook drives the **host** daemon.

That has a consequence for paths. The sibling containers compose starts declare bind mounts
like `./:/var/www/`, and the host daemon resolves them — a path that only exists inside the
`vhs` container silently becomes an empty, freshly created directory in the sibling. So the
tapes scaffold into `/tmp/specs-demo`, mounted at the *same path* on both sides:

```yaml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock
  - /tmp/specs-demo:/tmp/specs-demo
```

A fixed path rather than the checkout's own also keeps machine-specific strings out of the
frames — `git init` prints the absolute path of the repository it creates, and
`/tmp/specs-demo/acme/.git/` is the same on anyone's machine.

The socket is root-owned while the service runs under the host uid, hence `group_add: ["0"]`
on the service. `HOME` is set to `/tmp` for the same reason: the host uid has no entry in the
image's `/etc/passwd`, and `ttyd` needs a writable home.

## The tapes

### `docs/demo/use.tape`

`specs use specsnl/specs-laravel-project ./acme` — the prompt sequence answered on camera, the
hook-confirmation prompt, the four hooks running, then `ls acme` showing the result. The
answers pick `AddE2E: Yes`, so the listing ends with `e2e/` and `playwright.config.ts` next to
the `postgres/` directory the `Database` answer selected: the frame shows conditional files
being decided by the answers, not just files appearing.

The tape's hidden preamble warms the hook's image and npm caches off camera, so the recorded
run shows the hooks' output instead of pull progress bars. It also `git init`s the scratch
directory, because `task` in the generated project prints two `fatal: not a git repository`
lines when nothing above it is a repo — a real user scaffolds inside a workspace.

### `docs/demo/template.tape`

The registry flow: `specs template download` → `specs template save` → `specs template list`
→ `specs template use` → a commit that moves the saved template's source ahead →
`specs template update` → `specs template list` again.

Two entries rather than one, because the table is the point: the `Repository` column shows a
remote `owner/repo` and a `~`-collapsed local path side by side, and the `Status` column only
means something once one of them is out of date. The closing `list` is what shows
`update: <sha>` landing in the cached status.

## Shared settings

Both tapes set the same shell, font, padding, typing speed and window bar, so the two frames
match:

```text
Set Shell bash
Set FontFamily "JetBrains Mono"
Set FontSize 16
Set Padding 20
Set TypingSpeed 45ms
Set WindowBar Colorful
```

`Set Width` and `Set Height` are chosen **per tape** and justified in a comment there, from
the widest line and the tallest output the recorded commands actually produce:

| Tape            | Size        | Driven by                                                                                   |
|-----------------|-------------|---------------------------------------------------------------------------------------------|
| `use.tape`      | 1280 × 1120 | The 109-character `ProjectDescription` default; the 11-field prompt form fits on one screen |
| `template.tape` | 1000 × 600  | The 87-column `specs template list` table; `update` and `list` visible back to back         |

Anything narrower wraps mid-word, or folds the table.

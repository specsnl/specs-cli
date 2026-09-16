<!-- markdownlint-disable MD033 -->
<p align="center">
  <img src="docs/static/logo.svg" width="200" alt="Specs CLI">
  <h1 align="center">Specs CLI</h1>
  <p align="center"><strong>Documentation:</strong> <a href="https://cli.specs.dev">cli.specs.dev</a></p>
</p>
<!-- markdownlint-enable MD033 -->

Scaffold a project from a template — a git repository or a directory on disk.

A template is an ordinary tree of files plus a `project.yml` that declares the variables it needs.
`specs` asks for them, renders the tree, and runs the template's hooks. Templates you reuse can be
registered under a name, and `specs` tracks whether each one has moved ahead of what you have.

![specs use — answering the prompts and running the template's hooks](docs/static/demo/use.gif)

```yaml
# project.yml — the value is the default, the type decides the prompt
projectName: "my-app"
useDocker: false
license:
  - MIT
  - Apache-2.0

computed:
  packagePath: "github.com/{{ username }}/{{ .projectName }}"

hooks:
  post-use:
    - git init
```

```sh
specs use specsnl/my-template ./my-project          # one-off: fetch, render, discard
specs template download specsnl/my-template mine    # or register it once…
specs template use mine ./my-project                # …and reuse it by name
specs template list                                 # what's registered, and what has updates
```

What it does, in one list:

- **Renders a whole tree**, file names included, with 200+
  [Sprout](https://github.com/go-sprout/sprout) functions available. Files can be conditional,
  copied verbatim, or given explicit permissions.
- **Prompts only where something can answer.** With stdin not a terminal, a missing value fails
  immediately and names itself instead of hanging a CI job. `--arg`, `--values` and `--use-defaults`
  make a run unattended.
- **Keeps templates current.** `template list` reports each one's status — remote templates against
  their git remote, saved ones against their source directory — and `template upgrade` applies it.
- **Scripts cleanly.** stdout carries the answer, stderr the narration, so `2>/dev/null` leaves
  exactly the data. `--output json` makes it NDJSON.

---

## Install

```sh
brew install specsnl/tap/specs
```

Or `go install github.com/specsnl/specs-cli@latest`, or download a binary for your platform from the
[releases page](https://github.com/specsnl/specs-cli/releases).

Release candidates are a separate, opt-in cask that tracks every tag, prereleases included:

```sh
brew install specsnl/tap/specs@rc
```

Both casks provide a `specs` command and cannot be installed side by side — `brew uninstall specs`
before installing `specs@rc`, and the other way round.

Or run it from the official image, published for `linux/amd64` and `linux/arm64`:

```sh
docker run --rm -it -v "$PWD:/work" ghcr.io/specsnl/specs-cli use specsnl/my-template ./my-project
```

`-it` is what lets it prompt, and on a host where you are not uid 1000 add
`--user "$(id -u):$(id -g)" --env HOME=/tmp` so the scaffolded files come out yours. The image
carries `bash`, `git`, `task` and the `docker` CLI so a template's hooks have something to run. The
[installation docs](https://cli.specs.dev/docs/installation/) cover the rest — tags, the template
registry volume, hooks that start containers, and SSH sources.

---

## Getting started

Point `specs use` at any template and it will ask you the rest:

```sh
specs use specsnl/my-template ./my-project
```

For a worked example, the
[Laravel project tutorial](https://cli.specs.dev/docs/tutorials/laravel-project/) runs a real
template end to end — prompts, computed values, conditional files and hooks — and finishes with the
app running in the browser.

Writing your own starts with a `project.yml`. That file, every command and flag, the template
functions, scripting and CI, and how it is built are all on the same site:
**[cli.specs.dev](https://cli.specs.dev)**.

---

## Contributing

Every command runs through [Task](https://taskfile.dev), which wraps the Docker Compose services
that pin the Go and tooling versions — so a check runs the same way locally as it does in CI. No
local Go installation needed. Run `task --list` for the full set.

```sh
task dc:build     # build the images once
task build        # build the binary for the current platform
task test         # run the unit tests
task image:smoke  # build the published runtime image and check it
task lint:docker  # lint the Dockerfile with hadolint
```

The `Dockerfile` serves both purposes, and only one of its stages ships. `builder-download`,
`export` and `vhs` are development tools — they back `task test`, `task build` and
`task demo:record:*` respectively, and compose selects each by name. `debian` is the image
published to `ghcr.io/specsnl/specs-cli`, and it is the last stage in the file so a bare
`docker build .` produces it rather than a dev tool.

With Go 1.26+ installed you can bypass the container entirely — `go build ./...`, `go test ./...` —
but CI always runs through Docker and the Taskfile, so that is the source of truth.

Conventions, workflow, and the house rules that reviews are held to: [AGENTS.md](./AGENTS.md), and
the execution rules in
[`.github/instructions/executing-commands.md`](.github/instructions/executing-commands.md).

---

## License

MIT — see [LICENSE](./LICENSE).

---
title: specs use
weight: 1
---

A one-off command: fetch a template from any source, execute it into `<target-dir>`, then discard the download. Nothing is saved to the registry. For templates you'll reuse, use `specs template download` instead.

```sh
specs use specsnl/go-service ./new-service
specs use ./local-template ./output --use-defaults
specs use specsnl/go-service ./new-service --arg projectName=my-service
```

## Source formats

| Format                            | Example                                     |
|-----------------------------------|---------------------------------------------|
| GitHub shorthand (default branch) | `specsnl/go-service`                        |
| GitHub shorthand with branch      | `specsnl/go-service:main`                   |
| GitHub shorthand with tag         | `specsnl/go-service:v1.2.0`                 |
| Full HTTPS URL                    | `https://github.com/specsnl/go-service`     |
| Full HTTPS URL with `.git` suffix | `https://github.com/specsnl/go-service.git` |
| SCP-style SSH URL                 | `git@github.com:specsnl/go-service`         |
| Explicit SSH URL                  | `ssh://git@github.com/specsnl/go-service`   |
| Relative local path               | `./local-template`                          |
| Absolute local path               | `/home/user/templates/go-service`           |
| Explicit local path prefix        | `file:./local-template`                     |

GitHub is the default host, so a bare `owner/repo` needs no prefix; any other host is named in full. The shorthand `owner/repo:ref` accepts any git ref — a branch name, a tag, or a full commit SHA. HTTPS and SSH sources always use the repository's default branch.

The shorthand is matched last, after every URL and path form, so it never captures a source that was already valid. One consequence: **a relative path needs its leading `./`** — a bare `templates/go` is read as a repository, not a directory.

The older `github:owner/repo` form is still accepted as a deprecated alias.

## Flags

| Flag                  | Description                                                                                                                                |
|-----------------------|--------------------------------------------------------------------------------------------------------------------------------------------|
| `--values <file>`     | Load variable values from a JSON or YAML file (`.yaml`/`.yml` → YAML, otherwise JSON)                                                      |
| `--arg <key=value>`   | Set a single variable (repeatable); typed by the variable's default, see below                                                             |
| `--use-defaults`      | Accept all defaults without prompting                                                                                                      |
| `--no-hooks`          | Skip pre/post-use hooks                                                                                                                    |
| `--continue-on-error` | Warn instead of aborting on a render error: a file whose content fails is copied verbatim, a file or directory whose name fails is skipped |

With `--continue-on-error`, a run that hit render errors ends by saying how to see all of them:
`specs template validate ./path` for a local source, or to clone the repository and validate the
clone for a remote one.

## Typed `--arg` values

A value given with `--arg` takes the type of the variable's default in the project file, so it
behaves in a template exactly like the default it replaces:

| Default in the project file | `--arg` accepts                                                     | Example                    |
|-----------------------------|---------------------------------------------------------------------|----------------------------|
| boolean (`AddE2E: false`)   | `true`, `false`, `1`, `0` (and the other `strconv.ParseBool` forms) | `--arg AddE2E=false`       |
| integer (`Port: 3306`)      | an integer                                                          | `--arg Port=5432`          |
| float (`Ratio: 0.5`)        | a number                                                            | `--arg Ratio=0.75`         |
| list of options (a select)  | one of the options                                                  | `--arg License=MIT`        |
| string, or no such variable | anything, kept as a string                                          | `--arg ProjectName=my-app` |

A value that does not fit — `--arg AddE2E=nope`, `--arg Port=many`, an option that is not in the
list — is an error that names the argument, instead of a string that a template would read as
`true`. A `--values` file needs none of this: JSON and YAML keep their own types.

## Running without a terminal

A prompt needs someone to answer it. When stdin is not a terminal — a CI job, a cron entry, or
`specs use … < /dev/null` — specs refuses to prompt instead of blocking on a read nobody will
answer, and says which values it could not ask for:

```console
$ specs use specsnl/go-service ./out < /dev/null
error cannot prompt for values: stdin is not a terminal
missing values for: license, project_name
provide them with --arg Key=Value, with --values, or take the schema defaults with --use-defaults
```

Supply every variable the template needs and the command never reaches a prompt, so it runs
unattended. `--non-interactive` forces the same refusal at a terminal, which is how you check
that a command will survive CI before pushing it.

A remote template's **hook confirmation** is treated differently: with nothing to confirm it,
the answer is taken as "no". The template is still applied and only its hooks are skipped, with a
warning saying so. Pass `--yes` to run them unattended.

The form itself is drawn on **stderr**, never on stdout — so a prompt never lands in the file
`specs use … > out.txt` writes, and never corrupts `--output json`.

---
title: Scaffold a Laravel project
weight: 1
prev: /docs/tutorials
next: /docs/commands
---

[`specsnl/specs-laravel-project`](https://github.com/specsnl/specs-laravel-project) is a real
template that exercises most of what Specs can do: eleven prompts, nine computed values, conditional
files, a verbatim escape, custom delimiters, and four `post-use` hooks. This page runs it end to
end — from an empty directory to a Laravel app in the browser — and then registers it so the second
project costs one command.

Each step links to the reference page that owns the detail, rather than repeating it here.

## 1. Prerequisites

- **Docker** — [OrbStack](https://orbstack.dev/download) on macOS. The generated project runs its
  whole stack in containers, and the template's hooks use Docker too.
- **[Task](https://taskfile.dev/installation/)** — the generated project's entrypoint for everything.
- **[Homebrew](https://brew.sh)** — how this page installs `specs`. The
  [Installation](/docs/installation/) page covers the other routes.

## 2. Install specs

```sh
brew install specsnl/tap/specs
specs --version
```

To try an unreleased build first, `specs@rc` is a separate cask that follows every tag —
see [Installation](/docs/installation/#release-candidates).

## 3. Scaffold the project

```sh
specs use specsnl/specs-laravel-project ./acme
```

![specs use — answering the prompts and running the template's hooks](/demo/use.gif)

`specs use` fetches, renders and discards: nothing is registered, and the only thing left behind is
`./acme`. Bare `owner/repo` is shorthand for a GitHub repository; the `github:specsnl/…` form the
template's own README still uses means the same thing.

The template is remote and it defines hooks, so before running them `specs` prints the commands and
asks. That prompt is not a formality — see
[Hooks run other people's code](/docs/installation/#hooks-run-other-peoples-code).

## 4. The prompts

Eleven fields, asked as one form in alphabetical order. The answers in the third column are the ones
this page uses; everything after here assumes them.

| Prompt               | Type    | Answered with             | What it controls                                                  |
|----------------------|---------|---------------------------|-------------------------------------------------------------------|
| `AddBugSnag`         | confirm | No                        | Bugsnag in `composer.json`, `bootstrap/` and `config/logging.php` |
| `AddE2E`             | confirm | **Yes**                   | `e2e/`, `playwright.config.ts` and the `e2e` taskfile             |
| `AddFilament`        | confirm | No                        | Filament in `composer.json`, `bootstrap/providers.php` and `User` |
| `ComposerLicense`    | select  | `MIT`                     | `license` in `composer.json`                                      |
| `Database`           | select  | `postgres`                | The database service, its directory, and six computed values      |
| `IssuePrefix`        | text    | `ACME`                    | The PR title pattern in `.github/workflows/pr-title-checker.yml`  |
| `PhpVersion`         | select  | `8.5`                     | The PHP image tag, `composer.json` and the CI PHP setup           |
| `ProjectDescription` | text    | the default               | `README.md` and `composer.json`                                   |
| `ProjectName`        | text    | `Acme Storefront`         | `APP_NAME`, the README title, `package.json`                      |
| `ProjectShortName`   | text    | `acme`                    | `COMPOSE_PROJECT_NAME`, the database name, the OrbStack domains   |
| `RepoName`           | text    | `specsnl/acme-storefront` | `composer.json` name and the GitHub rulesets                      |

The type decides the widget: a string prompts for text, a bool for yes/no, a list for a choice of
its items. [The project file](/docs/project-yaml/) covers the mapping.

## 5. What those answers changed

This is where the template stops being a file copy.

**Two answers became nine values.** `Database: postgres` and `ProjectShortName: acme` fan out
through the `computed` block into the generated `.env`:

```sh
DB_CONNECTION="pgsql"
DB_HOST="postgres"
DB_PORT="5432"
DB_DATABASE="${POSTGRES_DB}"   # POSTGRES_DB="acme" — ProjectShortName through toSnakeCase
```

`DbOrbStackDomain` becomes `db.acme.local` in the README's service table, and `PhpVersion: 8.5`
selects `Php85DockerTag`, which is why `compose.yml` pins
`ghcr.io/specsnl/php85/builder_nodejs:0.5.7`. None of these were prompted for.
See [Computed values](/docs/architecture/computed-values/).

**Answering Yes to `AddE2E` wrote three paths that otherwise would not exist.** The template names
them with the condition in the filename:

```text
template/[[ if .AddE2E ]]e2e[[ end ]]/example.spec.ts
template/[[ if .AddE2E ]]playwright.config.ts[[ end ]]
template/.taskfiles/[[ if .AddE2E ]]Taskfile.e2e.yml[[ end ]]
```

A name that renders empty is skipped along with everything under it. The same trick gives the
database its directory — `postgres/` is present, `mysql/` and `mariadb/` were never written. See
[Template engine](/docs/architecture/template-engine/#conditional-files-and-directories).

**`[[ … ]]`, not `{{ … }}`.** A Laravel project is full of Blade and JavaScript that
already uses braces, so the template sets `__delimiters` in its `project.yml` and takes the braces
back. Everything above reads the same, only the fence changed. See
[Template delimiters](/docs/template-structure/#template-delimiters).

**One filename is an expression that evaluates to a constant.** The project's `.gitignore` is
shipped as ``template/[[`.gitignore`]]`` — backticks are a raw string in Go template syntax, so the
name renders to exactly `.gitignore`, and the file is only a live ignore file once it lands in your
project rather than in the template's own checkout. Same mechanism as the conditional filenames
above, with nothing conditional about it.

**The four `post-use` hooks did the setup you would otherwise do by hand.** They wrote `.env` from
`.env.example`, formatted the markdown, and left `./acme` as a git repository with everything
staged — so the first commit is yours, not the template's.

## 6. Boot it

Everything from here is the generated project's own tooling; `specs` is done.

```sh
cd acme
task up
```

The first run builds the images, installs the Composer and npm dependencies, generates the app key,
runs the migrations and builds the assets, so it takes a while. Afterwards:

| Service     | With OrbStack             | Without OrbStack        |
|-------------|---------------------------|-------------------------|
| Application | <https://acme.local>      | <http://localhost:8080> |
| Mailpit     | <https://mail.acme.local> | <http://localhost:8025> |
| PostgreSQL  | `db.acme.local`           | `localhost:5432`        |
| Redis       | `redis.acme.local`        | `localhost:6379`        |

The host ports are only published when OrbStack is not the active Docker context — with OrbStack the
`*.local` domains are the way in. Open the application URL and the page title is `Acme Storefront`,
which is `ProjectName` arriving in `APP_NAME`.

`task --list` shows the rest; `task stop` keeps the state, `task down` discards it.

## 7. Reuse it from the registry

`specs use` discards the template each time. For a template you will reach for again, register it
once:

```sh
specs template download specsnl/specs-laravel-project laravel
```

![specs template — download, save, list, use, and update](/demo/template.gif)

From then on the name is the source, and the prompts are identical:

```sh
specs template use laravel ./second-project
```

`specs template list` shows what is registered and whether each entry has moved behind its remote:

```text
┌─────────────────────────────────────────────────────────────────────────────────┐
│ Name     Repository                     Version  Status      Created   Updated  │
├─────────────────────────────────────────────────────────────────────────────────┤
│ laravel  specsnl/specs-laravel-project  03a6250  up-to-date  just now  just now │
└─────────────────────────────────────────────────────────────────────────────────┘
```

`specs template update` re-checks that status against the remote, and `specs template upgrade` pulls
the newer commit down. Where the files live is on the [Storage](/docs/storage/) page.

## Next steps

{{< cards >}}
  {{< card link="/docs/commands" title="Commands" subtitle="Every command and flag, including the non-interactive forms for CI." >}}
  {{< card link="/docs/template-structure" title="Template Structure" subtitle="The layout to start from when writing your own template." >}}
  {{< card link="/docs/project-yaml" title="The project file" subtitle="Declaring variables, computed values and hooks." >}}
  {{< card link="/docs/installation" title="Running it in Docker" subtitle="The official image, the bind-mount contract, and unattended runs." >}}
{{< /cards >}}

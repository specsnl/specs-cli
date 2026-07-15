---
title: Template Functions
weight: 6
---

Templates have access to 200+ functions provided by [Sprout](https://github.com/go-sprout/sprout), plus a set of specs-specific functions.

## Specs functions

| Function         | Signature                                                                 | Description                                |
|------------------|---------------------------------------------------------------------------|--------------------------------------------|
| `hostname`       | `hostname` → `string`                                                     | System hostname                            |
| `username`       | `username` → `string`                                                     | Current OS username                        |
| `toBinary`       | `toBinary <int>` → `string`                                               | Integer to binary string                   |
| `formatFilesize` | `formatFilesize <bytes>` → `string`                                       | Human-readable file size (e.g. `"1.0 MB"`) |
| `password`       | `password <length> <digits> <symbols> <noUpper> <allowRepeat>` → `string` | Generate a secure random password          |

```text
Default registry: {{ hostname }}.azurecr.io
Author: {{ username }}
Secret key: {{ password 32 4 4 false false }}
```

## Sprout function categories

| Category        | Example functions                                                                                           |
|-----------------|-------------------------------------------------------------------------------------------------------------|
| **Strings**     | `toUpper`, `toLower`, `toPascalCase`, `toSnakeCase`, `toKebabCase`, `trim`, `replace`, `contains`, `repeat` |
| **Encoding**    | `base64Encode`, `base64Decode`, `toJson`, `fromJson`, `toYaml`, `fromYaml`                                  |
| **Regex**       | `regexMatch`, `regexFind`, `regexReplaceAll`                                                                |
| **Collections** | `list`, `dict`, `append`, `prepend`, `uniq`, `keys`, `values`, `merge`                                      |
| **Date & time** | `now`, `date`, `dateModify`, `dateAgo`, `duration`                                                          |
| **Identity**    | `uuidv4`, `uuidv5`                                                                                          |
| **Crypto**      | `sha256sum`, `sha1sum`, `md5sum`, `bcrypt`                                                                  |
| **Numeric**     | `add`, `sub`, `mul`, `div`, `mod`, `floor`, `ceil`, `round`                                                 |
| **Semver**      | `semver`, `semverCompare`                                                                                   |
| **Network**     | `getHostByName`                                                                                             |
| **Random**      | `randInt`, `randAlpha`, `randAlphaNum`, `randAscii`                                                         |
| **Reflection**  | `typeOf`, `kindOf`, `kindIs`                                                                                |
| **Environment** | `env`, `expandenv` *(disabled in `--safe-mode`)*                                                            |
| **Filesystem**  | `osBase`, `osDir`, `osExt` *(disabled in `--safe-mode`)*                                                    |

> **Note:** Sprout uses Go-convention camelCase names. If you're migrating from sprig, see the [rename table](https://docs.gosprout.dev) for the full mapping.

Full function reference: [docs.gosprout.dev](https://docs.gosprout.dev).

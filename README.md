# Sphere CLI

Sphere CLI (`sphere-cli`) is a small bootstrap tool for [Sphere](https://github.com/go-sphere/sphere) projects. Its main job is to create projects from templates, record the exact template revision, list available templates, rename module paths, and provide a few lightweight generation helpers.

It is intentionally not the primary build, deploy, or runtime orchestration tool. After a project is created, day-to-day work should happen through the template Makefile, Buf, Go, Docker, and other mature tools already used by the Go ecosystem.

## Installation

To install `sphere-cli`, ensure you have Go installed and run:

```shell
go install github.com/go-sphere/sphere-cli@latest
```

## Scope

`sphere-cli` is responsible for:

- creating projects from official or custom layout templates;
- recording the source layout and exact Git commit in `.sphere/layout.lock.json`;
- listing available project templates;
- renaming Go module paths after project creation;
- generating small service skeletons when convenient.

`sphere-cli` is not responsible for:

- building binaries;
- running tests and linters;
- generating all project artifacts;
- managing Docker or Kubernetes deployment;
- replacing `make`, `buf`, `go`, `docker`, `wire`, `swag`, or other focused tools.

Generated templates expose those workflows through `make` targets instead.

## Usage

The general syntax is:

```shell
sphere-cli [command] [flags]
```

For detailed information on any command:

```shell
sphere-cli [command] --help
```

## Commands

### `create`

Initializes a new Sphere project from a layout template.

**Usage:**

```shell
sphere-cli create --name <project-name> [--module <go-module-name>] [--layout <template-uri-or-name>]
```

**Flags:**

- `--name string`: Required project directory name.
- `--module string`: Optional Go module path. Defaults to the project name when omitted.
- `--layout string`: Optional layout name or custom template layout URI.

Official layout names are `standard` (the default), `simple`, `bun`, and
`telegram`. Official layouts are cloned from their configured Git ref so the
created project records an exact upstream commit. Legacy custom JSON using
`uri`, `mod`, and `path` remains supported, but ZIP-only sources do not produce
a synchronization lock.

**Example:**

```shell
sphere-cli create --name myproject --module github.com/myorg/myproject
cd myproject
make init
make run
```

### `create list`

Lists available project templates.

**Usage:**

```shell
sphere-cli create list
```

### `service`

Generates small service skeleton files. This is a convenience helper, not the main project generation pipeline.

The main repeatable generation workflow stays in the generated project Makefile:

```shell
make gen/db
make gen/proto
make gen/docs
make gen/wire
```

#### `service proto`

Generates a `.proto` file for a new service.

**Usage:**

```shell
sphere-cli service proto --name <service-name> [--package <package-name>]
```

**Flags:**

- `--name string`: Required service name.
- `--package string`: Package name for the generated `.proto` file. Default: `dash.v1`.

> The generated proto references `entpb.<Entity>` messages, so the entity must
> already exist as an Ent schema and be annotated for `entproto` generation.
> Like `service golang`, run this inside the project so `--name` is matched
> against the real schema types (see the golang section below).

#### `service golang`

Generates a Go service implementation skeleton.

**Usage:**

```shell
sphere-cli service golang --name <service-name> [--package <package-name>] [--mod <go-module-path>]
```

**Flags:**

- `--name string`: Required service name.
- `--package string`: Package name for the generated Go code. Default: `dash.v1`.
- `--mod string`: Go module path for generated imports. Default: `github.com/go-sphere/sphere-layout`.

**Prerequisites.** The generated skeleton calls APIs produced by the project's
own code generators (`entbind.CreateXxx`, `ent.Xxx.Create`, `s.render.Xxx`), so
the entity must already exist as an Ent schema and the project must have run
`make gen/proto` at least once. Run this command **inside the project
directory**: the entity name is then matched against the real Ent schema types,
which lets an undivided name like `keyvaluestore` resolve to the
`KeyValueStore` schema. Outside a project (no schema directory) the name is
Pascal-cased by inflection only, so pass multi-word entities in separated form
(`key_value_store` or `key-value-store`) so they are split reliably.

The generated skeleton is a plain CRUD starting point, not a finished service:
real entities typically add field validation, password hashing, permission
checks, and `IgnoreSetZeroField` options on update (see the CRUD services in
the standard layout for reference).

### `rename`

Performs a project-wide rename of the Go module path.

**Usage:**

```shell
sphere-cli rename --old <old-module> --new <new-module> [--target <directory>]
```

**Flags:**

- `--old string`: Required current Go module path.
- `--new string`: Required new Go module path.
- `--target string`: Root directory of the project to rename. Default: `.`.

## Recommended Workflow

Use the CLI only at project boundaries:

```shell
sphere-cli create --name myproject --module github.com/myorg/myproject
cd myproject
make init
```

Then use the project Makefile for normal development:

```shell
make gen/all
make run
make lint
make build
```

## Layout Updates

The CLI intentionally does not merge template updates. AI agents use
`.sphere/layout.json` for ownership boundaries and `.sphere/layout.lock.json`
for the exact base revision, then follow the synchronization protocol in the
standard layout's `docs/LAYOUT_CONTRACT.md`. The lock must advance only after
the merged project regenerates, tests, lints, and builds successfully.

## License

**Sphere** is released under the MIT license. See [LICENSE](LICENSE) for details.

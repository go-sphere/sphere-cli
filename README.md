# Sphere CLI

Sphere CLI (`sphere-cli`) is a command-line tool designed to streamline the development of [Sphere](https://github.com/go-sphere/sphere) projects. It helps you create new projects, generate service code, manage Protobuf definitions, and perform other common development tasks.


## Installation

To install `sphere-cli`, ensure you have Go installed and run the following command:

```shell
go install github.com/go-sphere/sphere-cli@latest
```


## Usage

The general syntax for `sphere-cli` is:

```shell
sphere-cli [command] [flags]
```

For detailed information on any command, you can use the `--help` flag:

```shell
sphere-cli [command] --help
```


## Commands

Here is an overview of the available commands.

---

### `create`

Initializes a new Sphere project with a default template.

**Usage:**
```shell
sphere-cli create --name <project-name> [--module <go-module-name>] [--layout <template-uri>]
```

**Flags:**
- `--name string`: (Required) The name for the new Sphere project.
- `--module string`: (Optional) The Go module path for the project.
- `--layout string`: (Optional) Custom template layout URI.

#### `create list`

List available project templates

**Usage:**
```shell
sphere-cli create list
```

---

### `entproto`

Converts Ent schemas into Protobuf (`.proto`) definitions. This command reads your Ent schema files and generates corresponding `.proto` files.

**Usage:**
```shell
sphere-cli entproto [flags]
```

**Flags:**
- `--path string`: Path to the Ent schema directory (default: `./schema`).
- `--proto string`: Output directory for the generated `.proto` files (default: `./proto`).
- `--all_fields_required`: Treat all fields as required, ignoring `Optional()` (default: `true`).
- `--auto_annotation`: Automatically add `@entproto` annotations to the schema (default: `true`).
- `--enum_raw_type`: Use `string` as the type for enums in Protobuf (default: `true`).
- `--skip_unsupported`: Skip fields with types that are not supported (default: `true`).
- `--time_proto_type string`: Protobuf type to use for `time.Time` fields. Options: `int64`, `string`, `google.protobuf.Timestamp` (default: `int64`).
- `--uuid_proto_type string`: Protobuf type to use for `uuid.UUID` fields. Options: `string`, `bytes` (default: `string`).
- `--unsupported_proto_type string`: Protobuf type to use for unsupported fields. Options: `google.protobuf.Any`, `google.protobuf.Struct`, `bytes` (default: `google.protobuf.Any`).
- `--import_proto string`: Define external Protobuf imports. Format: `path1,package1,type1;path2,package2,type2` (default: `google/protobuf/any.proto,google.protobuf,Any;`).

---

### `service`

Generates service code, including both Protobuf definitions and Go service implementations.

This command has two subcommands: `proto` and `golang`.

#### `service proto`

Generates a `.proto` file for a new service.

**Usage:**
```shell
sphere-cli service proto --name <service-name> [--package <package-name>]
```

**Flags:**
- `--name string`: (Required) The name of the service.
- `--package string`: The package name for the generated `.proto` file (default: `dash.v1`).

#### `service golang`

Generates the Go implementation for a service from its definition.

**Usage:**
```shell
sphere-cli service golang --name <service-name> [--package <package-name>] [--mod <go-module-path>]
```

**Flags:**
- `--name string`: (Required) The name of the service.
- `--package string`: The package name for the generated Go code (default: `dash.v1`).
- `--mod string`: The Go module path for the generated code (default: `github.com/go-sphere/sphere-layout`).

---

### `rename`

Performs a project-wide rename of the Go module path.

**Usage:**
```shell
sphere-cli rename --old <old-module> --new <new-module> [--target <directory>]
```

**Flags:**
- `--old string`: (Required) The current Go module name.
- `--new string`: (Required) The new Go module name.
- `--target string`: The root directory of the project to rename (default: `.`).

---

### Other Commands

- `completion`: Generates shell autocompletion scripts (for Bash, Zsh, etc.).
- `help`: Provides help for any command.


## License

**Sphere** is released under the MIT license. See [LICENSE](LICENSE) for details.
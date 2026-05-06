# shed-sidecar

`shed-sidecar` provides VM-local management services for `shed`.

It builds two binaries:

- `shed-sidecard`: a daemon intended to run under systemd on Ubuntu VMs. It exposes the `sidecar.v1.Sidecar` gRPC service from `github.com/pcarion/shed-proto` on `127.0.0.1:50051` and `/run/sidecar/sidecar.sock`.
- `shed-sidecar`: a Cobra CLI for querying a local `shed-sidecard`.

## Build

```sh
make build
```

Binaries are written to `bin/`.

To build release artifacts locally with GoReleaser:

```sh
make release-snapshot
```

GitHub Releases are published automatically when a version tag is pushed:

```sh
make tag-patch
```

Use `make tag-major`, `make tag-minor`, or `make tag-patch` to create and push the next `v<major>.<minor>.<patch>` tag. The target fetches existing tags, finds the latest semantic version tag, increments the requested version part, and pushes the new tag to `origin`.

The `GoReleaser` workflow creates the release and uploads the Linux `amd64` and `arm64` archives plus checksums.

### GitHub Token Permissions

The release workflow uses the built-in `GITHUB_TOKEN` provided automatically by GitHub Actions. No repository secret is required for the default setup.

Make sure the repository allows workflows to create releases:

1. Open the repository on GitHub.
2. Go to **Settings** -> **Actions** -> **General**.
3. Under **Workflow permissions**, select **Read and write permissions**.
4. Enable **Allow GitHub Actions to create and approve pull requests** only if another workflow needs it; this release workflow does not.
5. Save the settings.

The workflow also declares the required job permission explicitly:

```yaml
permissions:
  contents: write
```

That permission lets GoReleaser create GitHub Releases and upload release assets using:

```yaml
env:
  GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Use a personal access token only if releasing to a different repository or organization policy blocks the built-in token. In that case, create a fine-grained token with **Contents: Read and write**, save it as a repository secret such as `GORELEASER_TOKEN`, and update the workflow environment to use that secret.

## Configuration

`shed-sidecard` reads `/etc/sidecar/config.toml` by default. The installer writes `config.toml` to the persistent directory you provide and installs the systemd unit with `--config` pointing at that file.

Complete `config.toml` format:

```toml
# TCP port for the localhost gRPC listener.
# shed-sidecard always binds to 127.0.0.1.
port = 50051

# Unix socket path for same-VM clients.
socket_path = "/run/sidecar/sidecar.sock"

# SQLite database path for persisted sidecar state such as passwords.
# Relative paths are resolved relative to this TOML file.
database_path = "sidecar.db"

# Inclusive range used for idempotent network port allocation.
network_port_min = 20000
network_port_max = 29999

# Optional allow-list for systemd service status queries.
# Empty means any unit can be queried.
# Bare names such as "nginx" are treated as "nginx.service".
allowed_services = []
```

Example with an allow-list:

```toml
port = 50051
socket_path = "/run/sidecar/sidecar.sock"
database_path = "state/sidecar.db"
network_port_min = 20000
network_port_max = 29999
allowed_services = [
  "ssh.service",
  "nginx",
  "zitadel.service",
]
```

Fields:

- `port`: Localhost TCP port for the gRPC server. Defaults to `50051`.
- `socket_path`: Unix socket path. Defaults to `/run/sidecar/sidecar.sock`.
- `database_path`: SQLite database file. Defaults to `sidecar.db`. Relative paths are resolved relative to the directory containing `config.toml`.
- `network_port_min`: First port in the inclusive allocation range. Defaults to `20000`.
- `network_port_max`: Last port in the inclusive allocation range. Defaults to `29999`.
- `allowed_services`: Optional systemd unit allow-list for `ServiceStatus`. Empty means any unit can be queried. Entries may use full unit names like `nginx.service` or bare service names like `nginx`.

## CLI

`shed-sidecar` talks to the local `shed-sidecard` gRPC listener. By default it connects to `127.0.0.1:50051`; use `--address` to point it at a different local endpoint:

```sh
shed-sidecar --address 127.0.0.1:50051 <command>
```

### Service Status

Query one or more systemd units:

```sh
shed-sidecar status nginx.service ssh.service
```

Bare service names are accepted by the daemon and treated as `.service` units:

```sh
shed-sidecar status nginx
```

The normal output is a compact table with a state symbol, service name, active state, sub state, and description. Use `--verbose` to request and print raw `systemctl status` output:

```sh
shed-sidecar status --verbose nginx.service
```

### Docker Status

List Docker containers on the VM:

```sh
shed-sidecar docker status
```

The output includes container name, state, human-readable status, image, and short container ID. The daemon uses the Docker SDK and lists all containers, including stopped containers.

### Passwords

Create or return an idempotent password:

```sh
shed-sidecar password get zitadel admin 32 hex-lower
```

Read an existing password without creating it:

```sh
shed-sidecar password read zitadel admin
```

List stored passwords:

```sh
shed-sidecar password list
```

### Network Ports

Allocate or return an idempotent network port:

```sh
shed-sidecar network port get zitadel http
```

List stored network port allocations:

```sh
shed-sidecar network port list
```

`shed-sidecar network list` is also accepted as a shorthand.

### Params

Set a service parameter:

```sh
shed-sidecar param set zitadel issuer https://zitadel.example.com
```

Read a stored parameter:

```sh
shed-sidecar param get zitadel issuer
```

List stored parameters:

```sh
shed-sidecar param list
```

### PostgreSQL pg_hba.conf

Configure a pg_hba rule through the daemon:

```sh
shed-sidecar postgres pg-hba configure /etc/postgresql/16/main/pg_hba.conf host all app scram-sha-256 --client-address 10.0.0.0/24
```

### Key/Value Config Files

Set one or more missing keys in a configuration file:

```sh
shed-sidecar conf set /etc/example/app.conf equal port=5432 --value-type number
shed-sidecar conf set /etc/example/app.conf colon listen_address=127.0.0.1
```

Read a key from a configuration file:

```sh
shed-sidecar conf get /etc/example/app.conf equal port
```

### Version

Print the build version:

```sh
shed-sidecar version
```

## Passwords

`shed-sidecard` creates a SQLite database at `database_path` and initializes a `passwords` table with these columns:

- `service`
- `name`
- `value`
- `generationDate`
- `length`
- `type`

The `PasswordGet` RPC returns an existing password when `service_name`, `name`, `length`, and `type` match an existing row. A different `length` or `type` generates and stores a new password, preserving idempotency for repeated calls with the same request. `PasswordRead` returns a stored password by service/name without creating one. `PasswordList` returns all stored passwords grouped by service name.

The CLI forms are:

```sh
shed-sidecar password get <service name> <name> <length> <type>
shed-sidecar password read <service name> <name>
shed-sidecar password list
```

Supported password types are `lowercase`, `uppercase`, `digit`, `symbol`, `hex-lower`, `hex-upper`, and `uuid-v7`. Short aliases are also accepted: `a`, `A`, `1`, `@`, `h`, `H`, and `u7`.

Password generation policies:

- `a` / `lowercase`: lowercase letters only.
- `A` / `uppercase`: lowercase and uppercase letters, with at least one uppercase letter.
- `1` / `digit`: digits only.
- `@` / `symbol`: lowercase letters, uppercase letters, and special characters, with at least one of each. Special characters exclude `$`, `/`, `\`, `(`, and `)`.
- `h` / `hex-lower`: lowercase hexadecimal characters.
- `H` / `hex-upper`: uppercase hexadecimal characters.
- `u7` / `uuid-v7`: UUIDv7.

## Network Ports

`shed-sidecard` also initializes a `network_ports` table with these columns:

- `service`
- `name`
- `port`
- `generationDate`

`NetworkPortGet` returns the same stored port every time `service_name` and `name` match an existing row. New allocations find the first port in the configured inclusive range that is not already stored in `network_ports` and is not currently bound on the VM, then store it with unique constraints on `(service, name)` and `port`. If another process takes the selected port concurrently, the allocation retries up to three times.

The CLI forms are:

```sh
shed-sidecar network port get <service name> <name>
shed-sidecar network port list
```

## Params

`shed-sidecard` also initializes a `params` table with these columns:

- `service`
- `name`
- `value`
- `generationDate`

`ParamSet` stores or updates a parameter value for a `(service_name, name)` pair. `ParamGet` returns the stored value, and `ParamList` returns all stored parameters grouped by service name.

The CLI forms are:

```sh
shed-sidecar param set <service name> <name> <value>
shed-sidecar param get <service name> <name>
shed-sidecar param list
```

## PostgreSQL pg_hba.conf

`shed-sidecard` implements `ConfigurePgHbaConf` from `shed-proto`. The request maps to a pg_hba rule using the PostgreSQL column order:

- `local`: `type database users method options`
- `host`: `type database users address method options`

The daemon does not validate the semantic content of the pg_hba parameters. It builds the rule columns from the request, reads the existing file, strips comments, and checks whether a matching row already exists. Existing rows return `is_valid=true` and `is_new=false`.

When the row is missing, the daemon creates an archive directory beside `config.toml`, writes a backup named `yyyy_mm_dd_hh_mm_ss_<file name>`, appends the new rule to the bottom of the requested file, and returns `is_valid=true` and `is_new=true`.

If `file_path` is relative, it is resolved relative to the directory containing `config.toml`.

The CLI form is:

```sh
shed-sidecar postgres pg-hba configure <file path> <local|host> <database> <users> <method> [--client-address <address>] [--options <options>]
```

Pass multiple users as a comma-separated value such as `app,migrator`.

## Key/Value Config Files

`shed-sidecard` implements `ConfigureKeyValueConf` and `ConfigureGetKeyValue` from `shed-proto`.

Supported formats are:

- `space`: `key value`
- `equal`: `key = value`
- `colon`: `key : value`

For `equal` and `colon`, existing active lines may use spacing variations such as `key=value`, `key = value`, `key:value`, or `key : value`. Lines whose first non-whitespace character is `#` are comments and are ignored when checking whether a key is already active.

`ConfigureKeyValueConf` checks each requested key before writing. If the key is already set on an active line, that key is left unchanged. If the key is missing and there is a nearby descriptive comment like `# key ...`, the daemon inserts the new `key value` line immediately after that comment. Otherwise, it appends the new line to the bottom of the file.

If any key is added, the daemon creates an archive directory beside `config.toml`, writes a backup named `yyyy_mm_dd_hh_mm_ss_<file name>`, writes the updated file, and returns `is_valid=true` and `is_new=true`. If no key is added, it returns `is_valid=true` and `is_new=false`.

Values with type `number` are written without quotes. Values with type `string` are escaped and written as quoted strings.

The CLI forms are:

```sh
shed-sidecar conf set <file path> <space|equal|colon> <key=value> ... [--value-type <string|number>]
shed-sidecar conf get <file path> <space|equal|colon> <key>
```

If `file_path` is relative, it is resolved relative to the directory containing `config.toml`.

## Docker Status

`shed-sidecard` implements `DockerStatus` from `shed-proto` using the Docker SDK. It connects to Docker using the standard Docker environment configuration and API version negotiation, then calls the Docker Engine API to list all containers.

The CLI form is:

```sh
shed-sidecar docker status
```

The installer adds the `sidecar` user to the `docker` group when that group exists. Restart the service after Docker is installed or after group membership changes.

## Install From A Release

Download and unpack the archive for the target VM from the GitHub Release, then run `install.sh` from the unpacked directory:

```sh
tar -xzf shed-sidecar_<version>_linux_<arch>.tar.gz
cd shed-sidecar_<version>_linux_<arch>
sudo ./install.sh /opt/shed-sidecar
```

The release archive includes `shed-sidecard`, `shed-sidecar`, `install.sh`, and `README.md`. The script installs the binaries from its own directory, creates the `sidecar` system user, creates `<persistent-dir>/config.toml` if needed, generates `/etc/systemd/system/sidecar.service` configured to use that file, and enables the service.

# shed-sidecar

`shed-sidecar` provides VM-local management services for `shed`.

It builds two binaries:

- `sidecard`: a daemon intended to run under systemd on Ubuntu VMs. It exposes the `sidecar.v1.Sidecar` gRPC service from `github.com/pcarion/shed-proto` on `127.0.0.1:50051` and `/run/sidecar/sidecar.sock`.
- `sidecarctl`: a Cobra CLI for querying a local `sidecard`.

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

`sidecard` reads `/etc/sidecar/config.toml` by default. The installer writes `config.toml` to the persistent directory you provide and installs the systemd unit with `--config` pointing at that file.

Complete `config.toml` format:

```toml
# TCP port for the localhost gRPC listener.
# sidecard always binds to 127.0.0.1.
port = 50051

# Unix socket path for same-VM clients.
socket_path = "/run/sidecar/sidecar.sock"

# SQLite database path for persisted sidecar state such as passwords.
# Relative paths are resolved relative to this TOML file.
database_path = "sidecar.db"

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
- `allowed_services`: Optional systemd unit allow-list for `ServiceStatus`. Empty means any unit can be queried. Entries may use full unit names like `nginx.service` or bare service names like `nginx`.

## CLI

```sh
sidecarctl status nginx.service ssh.service
sidecarctl status --verbose nginx.service
sidecarctl password get zitadel admin 32 hex-lower
sidecarctl version
```

`sidecarctl status --verbose` sets `include_raw` on `ServiceStatusRequest` and prints the raw `systemctl status` output returned by `sidecard`.

## Passwords

`sidecard` creates a SQLite database at `database_path` and initializes a `passwords` table with these columns:

- `service`
- `name`
- `value`
- `generationDate`
- `length`
- `type`

The `PasswordGet` RPC returns an existing password when `service_name`, `name`, `length`, and `type` match an existing row. A different `length` or `type` generates and stores a new password, preserving idempotency for repeated calls with the same request.

The CLI form is:

```sh
sidecarctl password get <service name> <name> <length> <type>
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

## Install From A Release

Download and unpack the archive for the target VM from the GitHub Release, then run `install.sh` from the unpacked directory:

```sh
tar -xzf shed-sidecar_<version>_linux_<arch>.tar.gz
cd shed-sidecar_<version>_linux_<arch>
sudo ./install.sh /opt/shed-sidecar
```

The release archive includes `sidecard`, `sidecarctl`, `install.sh`, and `README.md`. The script installs the binaries from its own directory, creates the `sidecar` system user, creates `<persistent-dir>/config.toml` if needed, generates `/etc/systemd/system/sidecar.service` configured to use that file, and enables the service.

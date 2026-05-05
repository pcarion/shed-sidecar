# shed-sidecar

`shed-sidecar` provides VM-local management services for `shed`.

It builds two binaries:

- `sidecard`: a daemon intended to run under systemd on Ubuntu VMs. It exposes the `sidecar.v1.Sidecar` gRPC service from `github.com/pcarion/shed-proto` on `127.0.0.1:8443` and `/run/sidecar/sidecar.sock`.
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

`sidecard` reads `/etc/sidecar/config.toml` by default. The installer writes `config.toml` to the persistent directory you provide and installs the systemd unit with `--config` pointing at that file:

```toml
port = 8443
socket_path = "/run/sidecar/sidecar.sock"
allowed_services = []
```

When `allowed_services` is empty, any requested systemd unit can be queried. When it contains unit names, all other services return a per-service error status.

## CLI

```sh
sidecarctl status nginx.service ssh.service
sidecarctl status --verbose nginx.service
sidecarctl version
```

`sidecarctl status --verbose` sets `include_raw` on `ServiceStatusRequest` and prints the raw `systemctl status` output returned by `sidecard`.

## Install From A Release

Download and unpack the archive for the target VM from the GitHub Release, then run `install.sh` from the unpacked directory:

```sh
tar -xzf shed-sidecar_<version>_linux_<arch>.tar.gz
cd shed-sidecar_<version>_linux_<arch>
sudo ./install.sh /opt/shed-sidecar
```

The release archive includes `sidecard`, `sidecarctl`, `install.sh`, `sidecar.service`, and `README.md`. The script installs the binaries from its own directory, creates the `sidecar` system user, creates `<persistent-dir>/config.toml` if needed, installs `sidecar.service` configured to use that file, and enables the service.

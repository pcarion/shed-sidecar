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

The GitHub Actions workflow `GoReleaser` is manual only and can be run from the Actions tab.

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

## Install

On the target VM, build or copy `sidecard` into the repository root, then run:

```sh
sudo ./install.sh /opt/shed-sidecar
```

The script creates the `sidecar` system user, installs `/usr/local/bin/sidecard`, creates `<persistent-dir>/config.toml` if needed, installs `sidecar.service` configured to use that file, and enables the service.

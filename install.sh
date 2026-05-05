#!/usr/bin/env sh
set -eu

usage() {
  echo "usage: sudo ./install.sh <persistent-dir>" >&2
  echo "example: sudo ./install.sh /opt/shed-sidecar" >&2
}

if [ "$#" -ne 1 ]; then
  usage
  exit 2
fi

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
SIDECARD_BIN="$SCRIPT_DIR/sidecard"
SIDECARCTL_BIN="$SCRIPT_DIR/sidecarctl"

if [ ! -f "$SIDECARD_BIN" ]; then
  echo "sidecard binary not found next to install.sh: $SIDECARD_BIN" >&2
  exit 1
fi

PERSISTENT_DIR=$1
case "$PERSISTENT_DIR" in
  /*) ;;
  *)
    echo "persistent-dir must be an absolute path" >&2
    exit 2
    ;;
esac

case "$PERSISTENT_DIR" in
  *[[:space:]%]*)
    echo "persistent-dir must not contain whitespace or percent characters" >&2
    exit 2
    ;;
esac

CONFIG_FILE="$PERSISTENT_DIR/config.toml"

if [ "$(id -u)" -ne 0 ]; then
  echo "install.sh must be run as root" >&2
  exit 1
fi

if ! id sidecar >/dev/null 2>&1; then
  useradd --system --home-dir /var/lib/sidecar --create-home --shell /usr/sbin/nologin sidecar
fi

usermod -aG systemd-journal sidecar

install -m 0755 "$SIDECARD_BIN" /usr/local/bin/sidecard
if [ -f "$SIDECARCTL_BIN" ]; then
  install -m 0755 "$SIDECARCTL_BIN" /usr/local/bin/sidecarctl
fi

install -d -m 0750 -o root -g sidecar "$PERSISTENT_DIR"
if [ ! -f "$CONFIG_FILE" ]; then
  cat >"$CONFIG_FILE" <<'EOF'
port = 8443
socket_path = "/run/sidecar/sidecar.sock"
allowed_services = []
EOF
fi
chown root:sidecar "$CONFIG_FILE"
chmod 0640 "$CONFIG_FILE"

cat >/etc/systemd/system/sidecar.service <<EOF
[Unit]
Description=Sidecar VM Management Service
After=network.target dbus.socket

[Service]
ExecStart=/usr/local/bin/sidecard --config $CONFIG_FILE
Restart=on-failure
User=sidecar
Group=sidecar
RuntimeDirectory=sidecar
StateDirectory=sidecar
ConfigurationDirectory=sidecar

[Install]
WantedBy=multi-user.target
EOF
chmod 0644 /etc/systemd/system/sidecar.service

systemctl daemon-reload
systemctl enable --now sidecar

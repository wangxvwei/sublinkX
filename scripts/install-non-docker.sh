#!/usr/bin/env bash
set -Eeuo pipefail

REPO="${REPO:-wangxvwei/sublinkX}"
BRANCH="${BRANCH:-feature/multi-xui-sources-docker}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin/sublink}"
SERVICE_NAME="${SERVICE_NAME:-sublink}"
SUBLINK_PORT="${SUBLINK_PORT:-8000}"
GO_VERSION="${GO_VERSION:-1.22.12}"

log() {
  printf '[sublinkx-install] %s\n' "$*"
}

die() {
  printf '[sublinkx-install] ERROR: %s\n' "$*" >&2
  exit 1
}

need_root() {
  if [ "$(id -u)" != "0" ]; then
    die "please run as root"
  fi
  command -v systemctl >/dev/null 2>&1 || die "systemd is required"
}

detect_arch() {
  case "$(uname -m)" in
    x86_64 | amd64)
      ARCH="amd64"
      ;;
    aarch64 | arm64)
      ARCH="arm64"
      ;;
    *)
      die "unsupported architecture: $(uname -m)"
      ;;
  esac
}

install_packages() {
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update
    DEBIAN_FRONTEND=noninteractive apt-get install -y ca-certificates curl tar gzip git
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y ca-certificates curl tar gzip git
  elif command -v yum >/dev/null 2>&1; then
    yum install -y ca-certificates curl tar gzip git
  elif command -v apk >/dev/null 2>&1; then
    apk add --no-cache ca-certificates curl tar gzip git
  else
    log "package manager not detected; assuming curl, tar, gzip and git are already installed"
  fi
}

install_go_if_needed() {
  export PATH="/usr/local/go/bin:$PATH"
  if command -v go >/dev/null 2>&1; then
    log "using existing Go: $(go version)"
    return
  fi

  local tarball="/tmp/go${GO_VERSION}.linux-${ARCH}.tar.gz"
  local url="https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz"
  log "installing Go ${GO_VERSION} for linux/${ARCH}"
  curl -fL "$url" -o "$tarball"
  rm -rf /usr/local/go
  tar -C /usr/local -xzf "$tarball"
  ln -sf /usr/local/go/bin/go /usr/local/bin/go
  export PATH="/usr/local/go/bin:/usr/local/bin:$PATH"
  command -v go >/dev/null 2>&1 || die "Go installation failed"
}

prepare_dirs() {
  mkdir -p "$INSTALL_DIR/db" "$INSTALL_DIR/logs" "$INSTALL_DIR/template"
  chmod 755 "$INSTALL_DIR"
}

write_default_config() {
  local config="$INSTALL_DIR/db/config.yaml"
  if [ -f "$config" ]; then
    log "keeping existing config: $config"
    return
  fi

  local secret
  secret="$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c 31 || true)"
  if [ -z "$secret" ]; then
    secret="$(date +%s%N)"
  fi

  cat >"$config" <<EOF
# jwt_secret: JWT secret
# expire_days: token expiration days
# port: listen port
jwt_secret: ${secret}
expire_days: 14
port: ${SUBLINK_PORT}
EOF
  chmod 600 "$config"
}

download_and_build() {
  local build_root="/tmp/sublinkx-build-$$"
  local tarball="/tmp/sublinkx-${BRANCH//\//-}.tar.gz"
  local source_url="https://github.com/${REPO}/archive/refs/heads/${BRANCH}.tar.gz"

  rm -rf "$build_root"
  mkdir -p "$build_root"

  log "downloading source: ${REPO}@${BRANCH}"
  curl -fL "$source_url" -o "$tarball"
  tar -xzf "$tarball" -C "$build_root" --strip-components=1

  log "building SublinkX"
  (
    cd "$build_root"
    GOOS=linux GOARCH="$ARCH" go build -ldflags="-w -s" -o sublink main.go
  )

  if [ -f "$INSTALL_DIR/sublink" ]; then
    cp "$INSTALL_DIR/sublink" "$INSTALL_DIR/sublink.bak.$(date +%Y%m%d%H%M%S)"
  fi
  install -m 755 "$build_root/sublink" "$INSTALL_DIR/sublink"
  rm -rf "$build_root" "$tarball"
}

write_service() {
  cat >"/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=SublinkX Service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/sublink
Restart=on-failure
RestartSec=3
LimitNOFILE=1048576
Environment=GIN_MODE=release

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable "$SERVICE_NAME"
  systemctl restart "$SERVICE_NAME"
}

main() {
  need_root
  detect_arch
  install_packages
  install_go_if_needed
  prepare_dirs
  write_default_config
  download_and_build
  write_service

  log "installed successfully"
  log "service: systemctl status ${SERVICE_NAME}"
  log "url: http://SERVER_IP:${SUBLINK_PORT}"
  log "default login: admin / 123456"
}

main "$@"

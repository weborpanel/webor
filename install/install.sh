#!/usr/bin/env bash
# Webor installer (Milestone 3).
# Usage:
#   sudo ./install/install.sh [--yes] [--config install.yaml] [--bin-dir ./bin]
#   sudo ./install/install.sh --uninstall [--purge]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

YES=0
UNINSTALL=0
PURGE=0
SKIP_PTR=0
SKIP_FIREWALL=0
CONFIG_FILE=""
BIN_DIR=""
PREFIX="/usr/local"
PANEL_DOMAIN=""
PANEL_PORT=0
LISTEN_HOST="0.0.0.0"
DISABLE_HTTPS=1
EXPECTED_A=""

STATE_DIR="/var/lib/webor"
ETC_DIR="/etc/webor"
RUN_DIR="/run/webor"
SITES_ROOT="/srv/sites"
PANEL_USER="webor"
CADDY_USER="caddy"

log()  { printf '[webor-install] %s\n' "$*"; }
warn() { printf '[webor-install] WARN: %s\n' "$*" >&2; }
die()  { printf '[webor-install] ERROR: %s\n' "$*" >&2; exit 1; }

usage() {
  cat <<'EOF'
Usage: sudo ./install/install.sh [options]

  --yes                 Non-interactive; assume yes for prompts
  --config FILE         Load options from a simple key: value file
  --bin-dir DIR         Directory with webor, webor-helper, wbr (default: ./bin)
  --prefix DIR          Install prefix for binaries (default: /usr/local)
  --panel-domain HOST   Panel hostname (optional)
  --panel-port N        First-run listen port (0 = random high port)
  --listen-host ADDR    Bind address (default: 0.0.0.0)
  --enable-https        Prefer HTTPS once DNS is ready (default: disabled for first-run)
  --expected-a IPS      Comma-separated public IPv4s for DNS checklist
  --skip-ptr            Skip PTR / forward-confirmed reverse DNS check
  --skip-firewall-hint  Skip firewall detection output
  --uninstall           Stop services and remove units/binaries
  --purge               With --uninstall, also remove /var/lib/webor and /etc/webor
  -h, --help            Show this help
EOF
}

trim() {
  local s="$1"
  s="${s#"${s%%[![:space:]]*}"}"
  s="${s%"${s##*[![:space:]]}"}"
  printf '%s' "$s"
}

load_config() {
  local file="$1" key val
  [[ -f "$file" ]] || die "config not found: $file"
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line%%#*}"
    line="$(trim "$line")"
    [[ -z "$line" ]] && continue
    key="$(trim "${line%%:*}")"
    val="$(trim "${line#*:}")"
    val="${val%\"}"; val="${val#\"}"
    val="${val%\'}"; val="${val#\'}"
    case "$key" in
      panel_domain) PANEL_DOMAIN="$val" ;;
      panel_port) PANEL_PORT="$val" ;;
      listen_host) LISTEN_HOST="$val" ;;
      disable_https)
        case "$val" in true|1|yes|YES) DISABLE_HTTPS=1 ;; false|0|no|NO) DISABLE_HTTPS=0 ;; esac
        ;;
      expected_a) EXPECTED_A="$val" ;;
      bin_dir) BIN_DIR="$val" ;;
      prefix) PREFIX="$val" ;;
      skip_firewall_hint)
        case "$val" in true|1|yes|YES) SKIP_FIREWALL=1 ;; esac
        ;;
      skip_ptr)
        case "$val" in true|1|yes|YES) SKIP_PTR=1 ;; esac
        ;;
      yes)
        case "$val" in true|1|yes|YES) YES=1 ;; esac
        ;;
    esac
  done < "$file"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --yes) YES=1; shift ;;
    --config) CONFIG_FILE="$2"; shift 2 ;;
    --bin-dir) BIN_DIR="$2"; shift 2 ;;
    --prefix) PREFIX="$2"; shift 2 ;;
    --panel-domain) PANEL_DOMAIN="$2"; shift 2 ;;
    --panel-port) PANEL_PORT="$2"; shift 2 ;;
    --listen-host) LISTEN_HOST="$2"; shift 2 ;;
    --enable-https) DISABLE_HTTPS=0; shift ;;
    --expected-a) EXPECTED_A="$2"; shift 2 ;;
    --skip-ptr) SKIP_PTR=1; shift ;;
    --skip-firewall-hint) SKIP_FIREWALL=1; shift ;;
    --uninstall) UNINSTALL=1; shift ;;
    --purge) PURGE=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done

[[ "$(id -u)" -eq 0 ]] || die "run as root (sudo)"

if [[ -n "$CONFIG_FILE" ]]; then
  load_config "$CONFIG_FILE"
fi

if [[ -z "$BIN_DIR" ]]; then
  if [[ -d "${REPO_ROOT}/bin" ]]; then
    BIN_DIR="${REPO_ROOT}/bin"
  else
    BIN_DIR="./bin"
  fi
fi

confirm() {
  local prompt="$1"
  if [[ "$YES" -eq 1 ]]; then
    return 0
  fi
  local ans
  read -r -p "$prompt [y/N] " ans || true
  case "$ans" in y|Y|yes|YES) return 0 ;; *) return 1 ;; esac
}

# --- preflight ------------------------------------------------------------

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

preflight_os() {
  require_cmd systemctl
  require_cmd curl
  require_cmd openssl
  require_cmd install
  require_cmd id
  if ! systemctl is-system-running >/dev/null 2>&1; then
    # degraded/starting still OK for install; missing systemd is not
    if ! [[ -d /run/systemd/system ]]; then
      die "systemd is required"
    fi
  fi
  if [[ -r /etc/os-release ]]; then
    # shellcheck disable=SC1091
    . /etc/os-release
    log "OS: ${PRETTY_NAME:-unknown}"
  fi
}

port_in_use() {
  local port="$1"
  if command -v ss >/dev/null 2>&1; then
    ss -ltn "( sport = :$port )" 2>/dev/null | grep -q ":$port" && return 0
  fi
  if command -v lsof >/dev/null 2>&1; then
    lsof -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1 && return 0
  fi
  return 1
}

preflight_ports() {
  local p
  for p in 80 443; do
    if port_in_use "$p"; then
      warn "port $p is already listening — Caddy/site TLS may conflict until freed"
    else
      log "port $p is free"
    fi
  done
}

detect_public_ip() {
  local ip=""
  ip="$(curl -4 -fsS --max-time 5 https://ifconfig.me 2>/dev/null || true)"
  if [[ -z "$ip" ]]; then
    ip="$(curl -4 -fsS --max-time 5 https://api.ipify.org 2>/dev/null || true)"
  fi
  if [[ -z "$ip" ]] && command -v dig >/dev/null 2>&1; then
    ip="$(dig +short -4 myip.opendns.com @resolver1.opendns.com 2>/dev/null | head -n1 || true)"
  fi
  printf '%s' "$(trim "$ip")"
}

ptr_for_ip() {
  local ip="$1"
  if command -v dig >/dev/null 2>&1; then
    dig +short -x "$ip" 2>/dev/null | sed 's/\.$//' | head -n1 || true
  elif command -v host >/dev/null 2>&1; then
    host "$ip" 2>/dev/null | awk '/pointer/ {print $NF}' | sed 's/\.$//' | head -n1 || true
  else
    printf ''
  fi
}

a_for_name() {
  local name="$1"
  if command -v dig >/dev/null 2>&1; then
    dig +short -4 A "$name" 2>/dev/null | head -n1 || true
  elif command -v getent >/dev/null 2>&1; then
    getent ahostsv4 "$name" 2>/dev/null | awk '{print $1; exit}' || true
  else
    printf ''
  fi
}

preflight_ptr() {
  local ip ptr fwd
  ip="$(detect_public_ip)"
  if [[ -z "$ip" ]]; then
    warn "could not detect public IPv4; set --expected-a and configure DNS manually"
    return 0
  fi
  log "public IPv4: $ip"
  if [[ -z "$EXPECTED_A" ]]; then
    EXPECTED_A="$ip"
  fi
  if [[ "$SKIP_PTR" -eq 1 ]]; then
    warn "skipping PTR check (--skip-ptr)"
    return 0
  fi
  ptr="$(ptr_for_ip "$ip")"
  if [[ -z "$ptr" ]]; then
    warn "no PTR for $ip — outbound mail and some providers prefer a matching reverse DNS"
    return 0
  fi
  log "PTR: $ip -> $ptr"
  fwd="$(a_for_name "$ptr")"
  if [[ -z "$fwd" ]]; then
    warn "PTR name $ptr has no A record (FCrDNS incomplete)"
    return 0
  fi
  if [[ "$fwd" != "$ip" ]]; then
    warn "FCrDNS mismatch: $ptr resolves to $fwd, expected $ip"
  else
    log "FCrDNS OK: $ip <-> $ptr"
  fi
}

detect_firewall() {
  [[ "$SKIP_FIREWALL" -eq 1 ]] && return 0
  log "firewall detection (guidance only; installer does not change rules):"
  if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -qi 'Status: active'; then
    log "  detected: ufw (active)"
    log "  allow panel temporarily:  sudo ufw allow ${PANEL_PORT}/tcp comment 'webor first-run'"
    log "  allow HTTP/HTTPS later:   sudo ufw allow 80,443/tcp"
  elif command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state 2>/dev/null | grep -qi running; then
    log "  detected: firewalld (running)"
    log "  allow panel:  sudo firewall-cmd --permanent --add-port=${PANEL_PORT}/tcp && sudo firewall-cmd --reload"
    log "  allow web:    sudo firewall-cmd --permanent --add-service=http --add-service=https && sudo firewall-cmd --reload"
  elif command -v nft >/dev/null 2>&1 && nft list ruleset 2>/dev/null | grep -q .; then
    log "  detected: nftables rules present — open TCP ${PANEL_PORT}, 80, and 443 for your policy"
  elif command -v iptables >/dev/null 2>&1 && iptables -S 2>/dev/null | grep -qv '^-P'; then
    log "  detected: iptables custom rules — open TCP ${PANEL_PORT}, 80, and 443 as needed"
  else
    log "  no common host firewall tooling detected (or inactive)"
  fi
  if [[ -d /proc/vz ]] || grep -qa container=lxc /proc/1/environ 2>/dev/null; then
    warn "container/VPS environment — also check the provider security group / network firewall"
  fi
}

# --- install / uninstall --------------------------------------------------

ensure_user() {
  local name="$1" home="$2"
  if id "$name" >/dev/null 2>&1; then
    log "user $name exists"
    return 0
  fi
  useradd --system --home-dir "$home" --create-home --shell /usr/sbin/nologin "$name"
  log "created system user $name"
}

# Site users (wbr_*) must traverse STATE_DIR to exec shared Node under runtimes/.
# Secrets stay 0640 on files; directory other+rx is intentional (not 0750).
# On SELinux Enforcing, Node binaries under runtimes must be bin_t (not var_lib_t)
# or systemd fails with status=203/EXEC.
ensure_state_layout() {
  install -d -m 0755 -o "$PANEL_USER" -g "$PANEL_USER" "$STATE_DIR"
  # Re-assert mode on upgrades: older installs used 0750 and broke Node backends.
  chmod 0755 "$STATE_DIR" || true
  install -d -m 0755 -o root -g root "${STATE_DIR}/runtimes"
  install -d -m 0755 -o root -g root "${STATE_DIR}/runtimes/node"
  install -d -m 0755 -o root -g root "$SITES_ROOT"
  install -d -m 0755 -o root -g root "$RUN_DIR" 2>/dev/null || true
  label_node_runtimes_selinux
}

label_node_runtimes_selinux() {
  local enforce node_root
  node_root="${STATE_DIR}/runtimes/node"
  [[ -d "$node_root" ]] || return 0
  [[ -d /sys/fs/selinux ]] || return 0
  command -v chcon >/dev/null 2>&1 || return 0
  enforce="$(cat /sys/fs/selinux/enforce 2>/dev/null || echo 0)"
  [[ "$enforce" == "1" ]] || return 0
  # Best-effort; missing trees or chcon failures must not abort install.
  find "$node_root" -type f -path '*/bin/*' -exec chcon -t bin_t {} + 2>/dev/null || true
  find "$node_root" -type f \( -name '*.so' -o -name '*.so.*' \) -exec chcon -t lib_t {} + 2>/dev/null || true
  log "SELinux Enforcing: labeled Node runtimes under ${node_root} (bin_t/lib_t)"
}

# Best-effort: Backups panel needs restic on PATH. Never fail the whole install.
install_restic() {
  if command -v restic >/dev/null 2>&1; then
    log "restic already installed: $(command -v restic)"
    return 0
  fi
  if command -v apt-get >/dev/null 2>&1; then
    log "installing restic via apt"
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -y >/dev/null || true
    if apt-get install -y restic; then
      log "restic installed: $(command -v restic)"
      return 0
    fi
    warn "apt install restic failed — install manually later for Backups"
    return 0
  fi
  if command -v dnf >/dev/null 2>&1; then
    log "installing restic via dnf (EPEL)"
    dnf install -y epel-release >/dev/null 2>&1 || true
    if dnf install -y restic; then
      log "restic installed: $(command -v restic)"
      return 0
    fi
    warn "dnf install restic failed — try: sudo dnf install -y epel-release restic"
    return 0
  fi
  warn "no apt/dnf — install restic manually for Backups"
}

install_binaries() {
  local bin
  for bin in webor webor-helper wbr; do
    [[ -x "${BIN_DIR}/${bin}" ]] || die "missing executable: ${BIN_DIR}/${bin} (build with: make build)"
  done
  install -d -m 0755 "${PREFIX}/bin"
  install -m 0755 "${BIN_DIR}/webor" "${PREFIX}/bin/webor"
  install -m 0755 "${BIN_DIR}/webor-helper" "${PREFIX}/bin/webor-helper"
  install -m 0755 "${BIN_DIR}/wbr" "${PREFIX}/bin/wbr"
  log "installed binaries to ${PREFIX}/bin"
  # RHEL/Alma sudo secure_path omits /usr/local/bin — keep wbr reachable as `sudo wbr`.
  if [[ "${PREFIX}/bin" != "/usr/bin" ]]; then
    ln -sfn "${PREFIX}/bin/wbr" /usr/bin/wbr
    log "linked /usr/bin/wbr -> ${PREFIX}/bin/wbr"
  fi
}

install_units() {
  install -d -m 0755 /etc/systemd/system
  install -m 0644 "${SCRIPT_DIR}/webor-helper.service" /etc/systemd/system/webor-helper.service
  install -m 0644 "${SCRIPT_DIR}/webor.service" /etc/systemd/system/webor.service
  # Caddy unit is installed only when caddy binary exists; otherwise skip.
  if command -v caddy >/dev/null 2>&1 || [[ -x /usr/bin/caddy ]]; then
    install -m 0644 "${SCRIPT_DIR}/webor-caddy.service" /etc/systemd/system/webor-caddy.service
    log "installed webor-caddy.service"
  else
    warn "caddy not found on PATH — skipped webor-caddy.service (install Caddy then re-run)"
  fi
  systemctl daemon-reload
}

random_high_port() {
  local p tries=0
  while [[ "$tries" -lt 40 ]]; do
    p=$((20000 + RANDOM % 20000))
    if ! port_in_use "$p"; then
      printf '%s' "$p"
      return 0
    fi
    tries=$((tries + 1))
  done
  die "could not find a free high port"
}

ensure_self_signed() {
  # Sets CERT_PATH / KEY_PATH when first-run HTTPS is requested.
  CERT_PATH=""
  KEY_PATH=""
  if [[ "$DISABLE_HTTPS" -eq 1 ]]; then
    return 0
  fi
  CERT_PATH="${ETC_DIR}/panel-selfsigned.crt"
  KEY_PATH="${ETC_DIR}/panel-selfsigned.key"
  if [[ -f "$CERT_PATH" && -f "$KEY_PATH" ]]; then
    return 0
  fi
  local cn="${PANEL_DOMAIN:-webor.local}"
  openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout "$KEY_PATH" -out "$CERT_PATH" -days 825 \
    -subj "/CN=${cn}" >/dev/null 2>&1
  chown "${PANEL_USER}:${PANEL_USER}" "$CERT_PATH" "$KEY_PATH"
  chmod 0640 "$CERT_PATH" "$KEY_PATH"
  log "generated self-signed cert for first-run TLS (${cn})"
}

write_panel_config() {
  local port="$1" token="$2"
  # /etc/webor must be traversable by caddy (other+x); secrets stay 0640.
  install -d -m 0751 -o "$PANEL_USER" -g "$PANEL_USER" "$ETC_DIR"
  ensure_state_layout

  if [[ ! -f "${ETC_DIR}/config.yaml" ]]; then
    cat > "${ETC_DIR}/config.yaml" <<'EOF'
version: 1
sites: []
EOF
    chown "${PANEL_USER}:${PANEL_USER}" "${ETC_DIR}/config.yaml"
    chmod 0640 "${ETC_DIR}/config.yaml"
  fi

  ensure_self_signed

  local https_flag="true"
  [[ "$DISABLE_HTTPS" -eq 0 ]] && https_flag="false"
  local args=(
    "-listen" "${LISTEN_HOST}:${port}"
    "-state-dir" "${STATE_DIR}"
    "-config" "${ETC_DIR}/config.yaml"
    "-caddy-json" "${ETC_DIR}/caddy.json"
    "-helper-socket" "${RUN_DIR}/helper.sock"
    "-setup-token" "${token}"
    "-disable-https=${https_flag}"
  )
  if [[ -n "$EXPECTED_A" ]]; then
    args+=("-expected-a" "${EXPECTED_A}")
  fi
  if [[ -n "$CERT_PATH" && -n "$KEY_PATH" ]]; then
    args+=("-tls-cert" "${CERT_PATH}" "-tls-key" "${KEY_PATH}")
  fi

  {
    echo "# Generated by install/install.sh — read by webor.service (systemd EnvironmentFile)"
    echo "# Panel domain (informational): ${PANEL_DOMAIN}"
    # Value is word-split by bash in ExecStart; keep it a single unquoted line.
    echo "WBR_PANEL_ARGS=${args[*]}"
    echo "WBR_PANEL_PORT=${port}"
    echo "WBR_SETUP_TOKEN=${token}"
  } > "${ETC_DIR}/panel.env"
  chmod 0640 "${ETC_DIR}/panel.env"
  log "wrote ${ETC_DIR}/panel.env"
}

write_bootstrap_caddy() {
  local json="${ETC_DIR}/caddy.json"
  if [[ -f "$json" ]]; then
    return 0
  fi
  # Minimal config so webor-caddy can start; panel reconcile replaces it.
  cat > "$json" <<'EOF'
{
  "admin": {"listen": "127.0.0.1:2019"},
  "apps": {
    "http": {
      "http_port": 8080,
      "https_port": 8443,
      "servers": {
        "srv0": {
          "listen": [":8080"],
          "routes": [{
            "handle": [{
              "handler": "static_response",
              "body": "webor-caddy-ok\n"
            }]
          }]
        }
      }
    }
  }
}
EOF
  chown "${PANEL_USER}:${PANEL_USER}" "$json"
  chmod 0644 "$json"
  install -d -m 0750 -o "${CADDY_USER}" -g "${CADDY_USER}" /var/lib/caddy 2>/dev/null || true
  log "wrote bootstrap ${json}"
}

enable_services() {
  write_bootstrap_caddy
  systemctl enable --now webor-helper.service
  systemctl enable --now webor.service
  if [[ -f /etc/systemd/system/webor-caddy.service ]] && command -v caddy >/dev/null 2>&1; then
    systemctl enable --now webor-caddy.service || warn "webor-caddy failed to start — check journalctl -u webor-caddy"
  fi
  log "enabled webor-helper + webor"
}

print_next_steps() {
  local port="$1" token="$2" scheme="http"
  [[ "$DISABLE_HTTPS" -eq 0 ]] && scheme="https"
  local host="${LISTEN_HOST}"
  [[ "$host" == "0.0.0.0" || "$host" == "::" ]] && host="$(detect_public_ip)"
  [[ -z "$host" ]] && host="SERVER_IP"
  cat <<EOF

════════════════════════════════════════════════════════════
 Webor first-run
════════════════════════════════════════════════════════════
  URL:          ${scheme}://${host}:${port}
  Setup token:  ${token}

  1. Open the URL and complete first-run (token + admin).
  2. Point A/AAAA for your panel domain to: ${EXPECTED_A:-SERVER_IP}
  3. Open firewall for TCP ${port} now; 80/443 when moving behind Caddy.
  4. Dangerous ops need step-up approval:
       wbr approve list
       wbr approve <id>
  5. Re-run this installer anytime; it is idempotent.

  Config:  ${ETC_DIR}/panel.env  (+ ${ETC_DIR}/config.yaml)
  State:   ${STATE_DIR}
  Units:   webor.service, webor-helper.service
════════════════════════════════════════════════════════════
EOF
}

do_uninstall() {
  log "stopping services"
  systemctl disable --now webor.service 2>/dev/null || true
  systemctl disable --now webor-helper.service 2>/dev/null || true
  systemctl disable --now webor-caddy.service 2>/dev/null || true
  rm -f /etc/systemd/system/webor.service \
        /etc/systemd/system/webor-helper.service \
        /etc/systemd/system/webor-caddy.service
  systemctl daemon-reload
  rm -f "${PREFIX}/bin/webor" "${PREFIX}/bin/webor-helper" "${PREFIX}/bin/wbr"
  if [[ "$PURGE" -eq 1 ]]; then
    if confirm "Purge ${STATE_DIR} and ${ETC_DIR}?"; then
      rm -rf "$STATE_DIR" "$ETC_DIR"
      log "purged state and config"
    fi
  else
    log "left ${STATE_DIR} and ${ETC_DIR} in place (use --purge to remove)"
  fi
  log "uninstall complete"
}

do_install() {
  preflight_os
  preflight_ports
  preflight_ptr

  if [[ "$PANEL_PORT" -eq 0 ]]; then
    PANEL_PORT="$(random_high_port)"
  fi
  detect_firewall

  if ! confirm "Install Webor into ${PREFIX} with panel on port ${PANEL_PORT}?"; then
    die "aborted"
  fi

  ensure_user "$PANEL_USER" "$STATE_DIR"
  if ! id "$CADDY_USER" >/dev/null 2>&1; then
    ensure_user "$CADDY_USER" /var/lib/caddy
  fi

  install_binaries
  install_units
  install_restic
  # Always (re)assert state dir mode + SELinux Node labels — even on idempotent re-run.
  ensure_state_layout

  local token
  if [[ -f "${ETC_DIR}/panel.env" ]] && grep -q 'WBR_SETUP_TOKEN=' "${ETC_DIR}/panel.env" 2>/dev/null; then
    log "existing ${ETC_DIR}/panel.env kept (idempotent re-run)"
    # shellcheck disable=SC1090
    token="$(grep '^WBR_SETUP_TOKEN=' "${ETC_DIR}/panel.env" | head -n1 | cut -d= -f2-)"
    PANEL_PORT="$(grep '^WBR_PANEL_PORT=' "${ETC_DIR}/panel.env" | head -n1 | cut -d= -f2- || echo "$PANEL_PORT")"
  else
    token="wbr-setup-$(openssl rand -hex 16)"
    write_panel_config "$PANEL_PORT" "$token"
  fi

  enable_services
  print_next_steps "$PANEL_PORT" "${token:-see ${ETC_DIR}/panel.env}"
}

if [[ "$UNINSTALL" -eq 1 ]]; then
  do_uninstall
  exit 0
fi

do_install

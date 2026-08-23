#!/usr/bin/env bash
# Caddy Proxy UI — install / manage script.
#
# Quickstart:
#   curl -fsSL https://raw.githubusercontent.com/chnthkksn/caddy-proxy-ui/main/contrib/install.sh | sudo bash -s -- install
#
# Prefer to review it first? Download, read it, then run:
#   curl -fsSL https://raw.githubusercontent.com/chnthkksn/caddy-proxy-ui/main/contrib/install.sh -o install.sh
#   less install.sh
#   sudo bash install.sh install
#
# Commands:
#   install            Install Caddy (if missing), caddy-ui, and the systemd service (default)
#   install-caddy      Install just the Caddy dependency, idempotent
#   run                Run caddy-ui in the foreground, no systemd service — for a quick trial
#   update             Update caddy-ui to the latest release, if one is available
#   reset-password     Reset the dashboard admin password (prompts, hides input)
#   uninstall [--purge]  Remove caddy-ui; --purge also deletes its SQLite data
#   status             Show systemd status for caddy and caddy-ui

set -euo pipefail

REPO="chnthkksn/caddy-proxy-ui"
BIN_PATH="/usr/local/bin/caddy-ui"
SERVICE_PATH="/etc/systemd/system/caddy-ui.service"
DATA_DIR="/var/lib/caddy-ui"
DB_PATH="$DATA_DIR/caddy-ui.db"

TMP_DIR="$(mktemp -d)"
cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

log() { echo "==> $*"; }

require_root() {
	if [ "$(id -u)" -ne 0 ]; then
		echo "This needs root — re-run with sudo." >&2
		exit 1
	fi
}

detect_suffix() {
	case "$(uname -m)" in
	x86_64) echo "linux_amd64" ;;
	aarch64 | arm64) echo "linux_arm64" ;;
	armv7l | armv6l) echo "linux_armv7" ;;
	*)
		echo "Unsupported architecture: $(uname -m)" >&2
		exit 1
		;;
	esac
}

install_caddy() {
	if command -v caddy >/dev/null 2>&1; then
		log "Caddy is already installed ($(caddy version))."
		return
	fi

	log "Installing Caddy..."
	if command -v apt-get >/dev/null 2>&1; then
		apt-get install -y debian-keyring debian-archive-keyring apt-transport-https curl gnupg
		curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' |
			gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
		curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' |
			tee /etc/apt/sources.list.d/caddy-stable.list >/dev/null
		chmod o+r /usr/share/keyrings/caddy-stable-archive-keyring.gpg
		chmod o+r /etc/apt/sources.list.d/caddy-stable.list
		apt-get update
		apt-get install -y caddy
	elif command -v dnf >/dev/null 2>&1; then
		dnf install -y dnf5-plugins 2>/dev/null || dnf install -y dnf-plugins-core
		dnf copr enable -y @caddy/caddy
		dnf install -y caddy
	else
		echo "Couldn't detect apt or dnf. Install Caddy manually: https://caddyserver.com/docs/install" >&2
		exit 1
	fi

	systemctl enable --now caddy
	log "Caddy installed and started."
}

install_binary() {
	local suffix url
	suffix="$(detect_suffix)"
	url="https://github.com/${REPO}/releases/latest/download/caddy-ui_${suffix}.tar.gz"

	log "Downloading caddy-ui ($suffix)..."
	curl -fsSL "$url" -o "$TMP_DIR/caddy-ui.tar.gz"
	tar xzf "$TMP_DIR/caddy-ui.tar.gz" -C "$TMP_DIR"
	install -m 0755 "$TMP_DIR/caddy-ui" "$BIN_PATH"
	log "Installed $("$BIN_PATH" version) to $BIN_PATH"
}

# latest_version resolves GitHub's /releases/latest redirect to read off the
# current tag, without needing jq or hitting the (rate-limited) API.
latest_version() {
	curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest" |
		sed 's#.*/tag/##'
}

install_service() {
	log "Installing systemd service..."
	cat >"$SERVICE_PATH" <<EOF
[Unit]
Description=Caddy Proxy UI
After=network.target caddy.service
Wants=caddy.service

[Service]
Type=simple
ExecStart=$BIN_PATH serve
Environment=CADDY_ADMIN_URL=http://localhost:2019
Environment=DB_PATH=$DB_PATH
DynamicUser=yes
StateDirectory=caddy-ui
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
	systemctl daemon-reload
	systemctl enable --now caddy-ui
}

print_access_info() {
	local ip
	ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
	ip="${ip:-<your-server-ip>}"
	echo
	log "caddy-ui is running. Open http://${ip}:8080 to create the administrator account."
}

cmd_install() {
	require_root
	install_caddy
	install_binary
	install_service
	print_access_info
}

cmd_update() {
	require_root
	if [ ! -x "$BIN_PATH" ]; then
		echo "caddy-ui isn't installed yet — run 'install' first." >&2
		exit 1
	fi

	local current latest
	current="$("$BIN_PATH" version)"
	latest="$(latest_version)"
	if [ -z "$latest" ]; then
		echo "Couldn't determine the latest version — check your network connection." >&2
		exit 1
	fi

	if [ "$current" = "$latest" ]; then
		log "Already up to date ($current)."
		return
	fi

	log "Updating caddy-ui: $current -> $latest"

	local was_running=0
	if systemctl is-active --quiet caddy-ui 2>/dev/null; then
		was_running=1
		systemctl stop caddy-ui
	fi

	install_binary

	if [ "$was_running" -eq 1 ]; then
		systemctl start caddy-ui
	fi

	log "Updated to $("$BIN_PATH" version)."
}

cmd_run() {
	require_root
	install_binary
	mkdir -p "$DATA_DIR"
	log "Running caddy-ui in the foreground (Ctrl+C to stop)..."
	DB_PATH="$DB_PATH" "$BIN_PATH" serve
}

cmd_reset_password() {
	require_root
	if [ ! -x "$BIN_PATH" ]; then
		echo "caddy-ui isn't installed yet — run 'install' first." >&2
		exit 1
	fi

	local was_running=0
	if systemctl is-active --quiet caddy-ui 2>/dev/null; then
		was_running=1
		log "Stopping caddy-ui..."
		systemctl stop caddy-ui
	fi

	local pass1 pass2
	if [ ! -e /dev/tty ]; then
		echo "No terminal available for a password prompt. Save this script and run" >&2
		echo "'reset-password' from a real terminal, or pipe a password directly into" >&2
		echo "the binary: echo 'newpassword' | DB_PATH=$DB_PATH $BIN_PATH reset-password" >&2
		[ "$was_running" -eq 1 ] && systemctl start caddy-ui
		exit 1
	fi
	read -rs -p "New dashboard password (min 8 chars): " pass1 </dev/tty
	echo
	read -rs -p "Confirm password: " pass2 </dev/tty
	echo

	if [ "$pass1" != "$pass2" ]; then
		echo "Passwords didn't match." >&2
		[ "$was_running" -eq 1 ] && systemctl start caddy-ui
		exit 1
	fi

	printf '%s\n' "$pass1" | DB_PATH="$DB_PATH" "$BIN_PATH" reset-password

	if [ "$was_running" -eq 1 ]; then
		log "Restarting caddy-ui..."
		systemctl start caddy-ui
	fi
}

cmd_uninstall() {
	require_root
	local purge=0
	[ "${1:-}" = "--purge" ] && purge=1

	systemctl stop caddy-ui 2>/dev/null || true
	systemctl disable caddy-ui 2>/dev/null || true
	rm -f "$SERVICE_PATH"
	systemctl daemon-reload
	rm -f "$BIN_PATH"

	if [ "$purge" -eq 1 ]; then
		rm -rf "$DATA_DIR"
		log "Removed caddy-ui and its data ($DATA_DIR)."
	else
		log "Removed caddy-ui. Data kept at $DATA_DIR — rerun with --purge to delete it too."
	fi
	log "Caddy itself was left installed — remove it separately if you no longer need it."
}

cmd_status() {
	echo "--- caddy ---"
	systemctl status caddy --no-pager 2>&1 | head -5 || true
	echo
	echo "--- caddy-ui ---"
	systemctl status caddy-ui --no-pager 2>&1 | head -5 || true
}

usage() {
	# Prints the leading '#'-comment block (the header above), whatever its
	# current length, stopping at the first non-comment line.
	sed -n '2,/^[^#]/p' "$0" | sed '$d' | sed 's/^# \{0,1\}//'
}

main() {
	local cmd="${1:-install}"
	[ $# -gt 0 ] && shift

	case "$cmd" in
	install) cmd_install ;;
	install-caddy)
		require_root
		install_caddy
		;;
	run) cmd_run ;;
	update) cmd_update ;;
	reset-password) cmd_reset_password ;;
	uninstall) cmd_uninstall "${1:-}" ;;
	status) cmd_status ;;
	-h | --help | help) usage ;;
	*)
		echo "Unknown command: $cmd" >&2
		usage
		exit 1
		;;
	esac
}

main "$@"

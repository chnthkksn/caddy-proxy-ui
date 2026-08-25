#!/usr/bin/env bash
# Caddy Proxy UI — install / manage script.
#
# Quickstart (shows an interactive menu — nothing runs until you pick an option):
#   curl -fsSL https://raw.githubusercontent.com/chnthkksn/caddy-proxy-ui/main/contrib/install.sh | sudo bash
#
# Prefer to review it first? Download, read it, then run:
#   curl -fsSL https://raw.githubusercontent.com/chnthkksn/caddy-proxy-ui/main/contrib/install.sh -o install.sh
#   less install.sh
#   sudo bash install.sh
#
# For scripting/automation, skip the menu by passing a command directly, e.g.:
#   curl -fsSL https://raw.githubusercontent.com/chnthkksn/caddy-proxy-ui/main/contrib/install.sh | sudo bash -s -- install
#
# The list of commands lives in usage() below, so that `help` prints the same
# text whether this file was downloaded or piped straight into bash.

set -euo pipefail

REPO="chnthkksn/caddy-proxy-ui"
BIN_PATH="/usr/local/bin/caddy-ui"
SERVICE_PATH="/etc/systemd/system/caddy-ui.service"
DATA_DIR="/var/lib/caddy-ui"
DB_PATH="$DATA_DIR/caddy-ui.db"
LOG_GROUP="caddy-ui-logs"
LOG_DIR="/var/log/caddy-ui"
ACCESS_LOG_PATH="$LOG_DIR/access.log"
CERT_GROUP="caddy-ui-certs"
# Where apt/dnf's Caddy package stores certificates by default (no
# XDG_DATA_HOME override in its systemd unit) — matches caddy-ui's own
# CADDY_STORAGE_PATH default in cmd/caddy-ui/serve.go.
CADDY_STORAGE_PATH="/var/lib/caddy/.local/share/caddy"

TMP_DIR="$(mktemp -d)"
cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

log() { echo "==> $*"; }

# have_tty checks /dev/tty is actually usable, not just present as a device
# node — the node can exist with no controlling terminal actually attached
# (e.g. some CI/sandboxed contexts), which a plain `[ -e /dev/tty ]` misses.
have_tty() {
	(: </dev/tty) 2>/dev/null
}

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

port_in_use() {
	local port="$1"
	if command -v ss >/dev/null 2>&1; then
		# Plain grep over the full listing rather than ss's own filter DSL —
		# fewer ways to get the syntax subtly wrong.
		ss -ltn 2>/dev/null | grep -q ":${port} "
	elif command -v netstat >/dev/null 2>&1; then
		netstat -ltn 2>/dev/null | grep -q ":${port} "
	else
		(exec 3<>"/dev/tcp/127.0.0.1/$port") 2>/dev/null
	fi
}

# check_ports fails fast, before touching anything, if a port we need is
# already bound by something else — rather than letting the user discover
# it later as a crash-looping systemd service. Skips ports already owned by
# our own (already-running) services, so re-running `install` stays fine.
check_ports() {
	local conflicts=()

	if ! systemctl is-active --quiet caddy 2>/dev/null; then
		for port in 80 443; do
			port_in_use "$port" && conflicts+=("$port (needed by Caddy)")
		done
	fi

	if ! systemctl is-active --quiet caddy-ui 2>/dev/null; then
		port_in_use 8080 && conflicts+=("8080 (needed by caddy-ui)")
	fi

	if [ "${#conflicts[@]}" -gt 0 ]; then
		echo "These ports are already in use by something else:" >&2
		printf '  - %s\n' "${conflicts[@]}" >&2
		echo "Free them first, or edit /etc/systemd/system/caddy-ui.service and your" >&2
		echo "Caddy config to use different ports." >&2
		exit 1
	fi
}

install_caddy() {
	if command -v caddy >/dev/null 2>&1; then
		log "Caddy is already installed ($(caddy version))."
	else
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
		log "Caddy installed."
	fi

	# "Installed" doesn't mean "running" — a previously-installed Caddy
	# could be stopped, or have no systemd unit at all (e.g. a manual
	# binary-only setup), and caddy-ui needs its Admin API actually up.
	if systemctl list-unit-files caddy.service >/dev/null 2>&1; then
		if systemctl is-active --quiet caddy; then
			log "Caddy's service is already running."
		else
			log "Caddy's service isn't running — starting it."
			systemctl enable --now caddy
		fi
	else
		echo "Warning: no 'caddy' systemd service found to manage. If Caddy was set" >&2
		echo "up manually, make sure its Admin API is reachable at http://localhost:2019" >&2
		echo "(Caddy's own default) or caddy-ui won't be able to push config to it." >&2
	fi

	setup_access_log_sharing
	setup_cert_storage_sharing
}

# ensure_acl_tools installs setfacl if it's missing. Needed for
# setup_cert_storage_sharing's default ACLs — a plain one-time chmod would
# only cover certificates that exist at install time, not ones Caddy issues
# later, since default ACLs (unlike chmod) apply automatically to new files
# a directory's owner creates after the fact.
ensure_acl_tools() {
	if command -v setfacl >/dev/null 2>&1; then
		return
	fi
	log "Installing 'acl' (needed for read-only certificate sharing)..."
	if command -v apt-get >/dev/null 2>&1; then
		apt-get install -y acl
	elif command -v dnf >/dev/null 2>&1; then
		dnf install -y acl
	fi
}

# setup_access_log_sharing lets Caddy (running as its own "caddy" system
# user) write access logs into a directory caddy-ui (running as a systemd
# DynamicUser) can read, without loosening either process's own permissions.
# Mechanism: a dedicated shared group, and a setgid directory so files Caddy
# creates automatically inherit that group regardless of Caddy's own primary
# group — see contrib/systemd/caddy-ui.service's SupplementaryGroups for the
# other half.
setup_access_log_sharing() {
	groupadd -f "$LOG_GROUP"

	mkdir -p "$LOG_DIR"
	chgrp "$LOG_GROUP" "$LOG_DIR"
	chmod 2775 "$LOG_DIR"

	# setgid above puts Caddy's log files in the shared group, but that alone
	# does not make them readable: Caddy creates them 0600, so group members
	# get nothing. A default ACL is what actually grants the read, and unlike
	# a chmod it also covers the new file Caddy opens after each rotation.
	ensure_acl_tools
	if command -v setfacl >/dev/null 2>&1; then
		setfacl -R -m "g:$LOG_GROUP:rX" "$LOG_DIR"
		setfacl -R -d -m "g:$LOG_GROUP:rX" "$LOG_DIR"
	else
		echo "Warning: couldn't install 'acl', so caddy-ui cannot read Caddy's" >&2
		echo "access log and the Logs & traffic page will report it as unavailable." >&2
	fi

	if ! id -u caddy >/dev/null 2>&1; then
		log "No 'caddy' system user found — skipping access log sharing setup."
		return
	fi

	if id -nG caddy | tr ' ' '\n' | grep -qx "$LOG_GROUP"; then
		return
	fi

	usermod -aG "$LOG_GROUP" caddy
	log "Added 'caddy' to the '$LOG_GROUP' group for access log sharing."
	if systemctl is-active --quiet caddy 2>/dev/null; then
		log "Restarting Caddy so the new group membership takes effect."
		systemctl restart caddy
	fi
}

# setup_cert_storage_sharing gives caddy-ui read-only access to Caddy's own
# certificate storage, so the Certificates page can show real expiry dates.
# Unlike setup_access_log_sharing, Caddy itself needs no changes here — it
# already owns these files — so this only needs default ACLs (setfacl -d),
# not a shared group with Caddy as a member. Default ACLs, unlike a one-time
# chmod, also cover certificates Caddy issues after this script runs.
setup_cert_storage_sharing() {
	ensure_acl_tools
	if ! command -v setfacl >/dev/null 2>&1; then
		echo "Warning: couldn't install 'acl' — the Certificates page won't be able" >&2
		echo "to read Caddy's certificate storage. Install the 'acl' package manually" >&2
		echo "and rerun 'install' to enable it." >&2
		return
	fi

	groupadd -f "$CERT_GROUP"

	# Created with the same ownership Caddy's package normally leaves it at
	# (root:root, 0755) if it doesn't exist yet — a fresh Caddy install
	# hasn't issued anything, so this directory may not exist until its
	# first certificate. The ACLs below apply as soon as it does.
	mkdir -p "$CADDY_STORAGE_PATH"

	# rX: read, and traverse-if-directory — never makes a regular file
	# executable. -R applies it to what's already there; -d makes it the
	# default ACL so anything Caddy creates under here later inherits it
	# automatically, with no re-run of this script required.
	setfacl -R -m "g:$CERT_GROUP:rX" "$CADDY_STORAGE_PATH"
	setfacl -R -d -m "g:$CERT_GROUP:rX" "$CADDY_STORAGE_PATH"
	log "Granted the '$CERT_GROUP' group read-only access to $CADDY_STORAGE_PATH."
}

# latest_version resolves GitHub's /releases/latest redirect to read off the
# current tag, without needing jq or hitting the (rate-limited) API.
latest_version() {
	curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest" |
		sed 's#.*/tag/##'
}

# install_binary downloads and installs caddy-ui, but does nothing if
# what's already installed already matches the latest release — makes
# `install` cheap and quiet to re-run instead of re-downloading every time,
# and is reused by `update`. Returns 2 (not an error, but "nothing to do")
# when it skipped, so callers can tell whether anything actually changed.
install_binary() {
	local suffix latest current
	suffix="$(detect_suffix)"
	latest="$(latest_version)"

	if [ -x "$BIN_PATH" ] && [ -n "$latest" ]; then
		current="$("$BIN_PATH" version 2>/dev/null || echo "")"
		if [ "$current" = "$latest" ]; then
			log "caddy-ui is already up to date ($current)."
			return 2
		fi
	fi

	log "Downloading caddy-ui ${latest:-latest} ($suffix)..."
	curl -fsSL "https://github.com/${REPO}/releases/latest/download/caddy-ui_${suffix}.tar.gz" \
		-o "$TMP_DIR/caddy-ui.tar.gz"
	tar xzf "$TMP_DIR/caddy-ui.tar.gz" -C "$TMP_DIR"
	# `install` replaces the destination file rather than editing it in
	# place, so this is safe to run even while the old binary is still
	# running as a service — it just won't take effect until restarted.
	install -m 0755 "$TMP_DIR/caddy-ui" "$BIN_PATH"
	log "Installed $("$BIN_PATH" version) to $BIN_PATH"
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
Environment=ACCESS_LOG_PATH=$ACCESS_LOG_PATH
Environment=CADDY_STORAGE_PATH=$CADDY_STORAGE_PATH
DynamicUser=yes
SupplementaryGroups=$LOG_GROUP $CERT_GROUP
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
	# Prefer the actual public IP — `hostname -I` lists every address the
	# host has, and on providers with a private/internal network interface
	# that's often not the first (or reachable) one.
	ip="$(curl -fsSL -4 --max-time 3 https://ifconfig.me 2>/dev/null || true)"
	if [ -z "$ip" ]; then
		ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
	fi
	ip="${ip:-<your-server-ip>}"
	echo
	log "caddy-ui is running. Open http://${ip}:8080 to create the administrator account."
	echo "If that doesn't load: check your firewall/cloud security group allows inbound"
	echo "TCP 8080 (and 80/443 for Caddy), and that ${ip} is this server's real reachable"
	echo "address — some providers list a private network IP first."
}

cmd_install() {
	require_root
	check_ports
	install_caddy
	install_binary || true # 2 just means "already current" — not an error
	install_service
	print_access_info
}

cmd_update() {
	require_root
	if [ ! -x "$BIN_PATH" ]; then
		echo "caddy-ui isn't installed yet — run 'install' first." >&2
		exit 1
	fi

	# Replacing the binary is safe even while the old version is still
	# running as a service — see install_binary.
	install_binary || true # 2 just means "already current", not an error

	# Re-apply the service configuration on every update, not only on a fresh
	# install. A release can introduce new environment variables or new
	# directories — the access log and certificate sharing below both did —
	# and an update that swapped the binary alone would leave it running
	# under a unit written before any of that existed. That is not a
	# hypothetical: it is how a working install broke on upgrade, with the
	# new binary falling back to a relative access-log path that DynamicUser
	# cannot create.
	setup_access_log_sharing
	setup_cert_storage_sharing
	install_service

	# install_service only enables the unit; an already-running service keeps
	# the old environment until it is actually restarted.
	systemctl restart caddy-ui
	log "caddy-ui is up to date and running the current service configuration."
}

cmd_run() {
	require_root
	if ! systemctl is-active --quiet caddy-ui 2>/dev/null && port_in_use 8080; then
		echo "Port 8080 is already in use by something else — stop it first," >&2
		echo "or set LISTEN_ADDR to run caddy-ui on a different port." >&2
		exit 1
	fi
	install_binary || true
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
	if ! have_tty; then
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
		rm -rf "$DATA_DIR" "$LOG_DIR"
		log "Removed caddy-ui and its data ($DATA_DIR, $LOG_DIR)."
	else
		log "Removed caddy-ui. Data kept at $DATA_DIR and $LOG_DIR — rerun with --purge to delete it too."
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

# usage must not read "$0": the documented way to run this is
# `curl ... | sudo bash -s -- <command>`, where $0 is "bash" and the script
# itself was consumed from stdin and is no longer readable anywhere.
usage() {
	cat <<'EOF'
Caddy Proxy UI — install / manage script.

Usage:
  sudo bash install.sh [command]
  curl -fsSL https://raw.githubusercontent.com/chnthkksn/caddy-proxy-ui/main/contrib/install.sh | sudo bash -s -- [command]

With no command, an interactive menu is shown and nothing runs until you pick
an option. Automated runs must name a command, since the menu waits for input.

Commands:
  install              Install Caddy (if missing), caddy-ui, and the systemd service
  install-caddy        Install just the Caddy dependency, idempotent
  run                  Run caddy-ui in the foreground, no systemd service — for a quick trial
  update               Update caddy-ui to the latest release, if one is available
  reset-password       Reset the dashboard admin password (prompts, hides input)
  uninstall [--purge]  Remove caddy-ui; --purge also deletes its SQLite data
  status               Show systemd status for caddy and caddy-ui
  help                 Show this message
EOF
}

# MENU_CMD is set by interactive_menu rather than returned via command
# substitution, so `exit` inside it (e.g. on "quit") actually exits the
# script instead of just a subshell.
MENU_CMD=""

interactive_menu() {
	if ! have_tty; then
		echo "No terminal available to show a menu. Pass a command directly instead," >&2
		echo "e.g.: curl ... | sudo bash -s -- install  (see 'help' for the full list)" >&2
		exit 1
	fi

	cat >/dev/tty <<'EOF'

Caddy Proxy UI

  1) Install              Caddy (if missing) + caddy-ui + systemd service
  2) Install Caddy only   just the dependency, idempotent
  3) Run in foreground    no systemd — quick trial
  4) Update               update caddy-ui to the latest release
  5) Reset password       reset the dashboard admin password
  6) Uninstall            remove caddy-ui (keeps its data)
  7) Status               systemd status for caddy and caddy-ui
  q) Quit, do nothing

EOF
	local choice
	read -rp "Choice [1]: " choice </dev/tty

	case "${choice:-1}" in
	1) MENU_CMD="install" ;;
	2) MENU_CMD="install-caddy" ;;
	3) MENU_CMD="run" ;;
	4) MENU_CMD="update" ;;
	5) MENU_CMD="reset-password" ;;
	6) MENU_CMD="uninstall" ;;
	7) MENU_CMD="status" ;;
	q | Q) exit 0 ;;
	*)
		echo "Unrecognized choice: $choice" >&2
		exit 1
		;;
	esac
}

main() {
	local cmd
	if [ $# -eq 0 ]; then
		interactive_menu
		cmd="$MENU_CMD"
	else
		cmd="$1"
		shift
	fi

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

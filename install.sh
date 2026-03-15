#!/usr/bin/env bash
# streamdeck-go installer
# Handles dotfiles-aware installation with interactive prompts.
set -euo pipefail

BINARY="streamdeck-go"
BIN_DIR="${HOME}/.local/bin"
SYSTEMD_USER="${HOME}/.config/systemd/user"
UDEV_RULE="/etc/udev/rules.d/99-streamdeck.rules"
XDG_CONFIG="${HOME}/.config"
DEFAULT_DOTFILES="${HOME}/dotfiles"

# ── Colours ────────────────────────────────────────────────────────────────────
if [[ -t 1 ]]; then
  BOLD='\033[1m'; DIM='\033[2m'
  RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
  BLUE='\033[0;34m'; CYAN='\033[0;36m'; MAGENTA='\033[0;35m'
  NC='\033[0m'
else
  BOLD=''; DIM=''; RED=''; GREEN=''; YELLOW=''
  BLUE=''; CYAN=''; MAGENTA=''; NC=''
fi

CHECK="${GREEN}✓${NC}"
CROSS="${RED}✗${NC}"
ARROW="${CYAN}❯${NC}"
WARN="${YELLOW}⚠${NC}"
DOT="${DIM}·${NC}"

# ── Helpers ────────────────────────────────────────────────────────────────────
nl()    { echo ""; }
step()  { echo -e "  ${ARROW} $1"; }
ok()    { echo -e "  ${CHECK} $1"; }
warn()  { echo -e "  ${WARN} $1"; }
info()  { echo -e "  ${DOT} $1"; }
error() { echo -e "  ${CROSS} ${RED}$1${NC}"; }
dim()   { echo -e "  ${DIM}$1${NC}"; }

prompt_yn() {
  local msg="$1" default="${2:-y}"
  local opts; [[ $default == "y" ]] && opts="Y/n" || opts="y/N"
  echo -ne "  ${ARROW} ${msg} ${DIM}[${opts}]${NC} "
  read -r _answer </dev/tty
  _answer="${_answer:-$default}"
  [[ $_answer =~ ^[Yy] ]]
}

prompt_input() {
  local msg="$1" default="$2"
  echo -ne "  ${ARROW} ${msg} ${DIM}[${default}]${NC}: "
  read -r _input </dev/tty
  echo "${_input:-$default}"
}

abspath() {
  # Expand ~, resolve to absolute path without requiring it to exist yet.
  local p="${1/#\~/$HOME}"
  echo "$p"
}

# ── Dependency detection ───────────────────────────────────────────────────────

detect_pkg_manager() {
  if   command -v pacman &>/dev/null; then echo "pacman"
  elif command -v apt    &>/dev/null; then echo "apt"
  elif command -v dnf    &>/dev/null; then echo "dnf"
  elif command -v zypper &>/dev/null; then echo "zypper"
  else echo "unknown"
  fi
}

# Returns 0 if hidapi dev headers are present (needed for cgo build).
hidapi_present() {
  pkg-config --exists hidapi-hidraw 2>/dev/null ||
  pkg-config --exists hidapi        2>/dev/null ||
  ldconfig -p 2>/dev/null | grep -q libhidapi
}

install_pkg() {
  local pm="$1"; shift
  local pkgs=("$@")
  case "$pm" in
    pacman) sudo pacman -S --needed --noconfirm "${pkgs[@]}" ;;
    apt)    sudo apt-get install -y "${pkgs[@]}" ;;
    dnf)    sudo dnf install -y "${pkgs[@]}" ;;
    zypper) sudo zypper install -y "${pkgs[@]}" ;;
  esac
}

# ── Header ─────────────────────────────────────────────────────────────────────
nl
echo -e "  ${BOLD}${CYAN}streamdeck-go${NC}  ${DIM}installer${NC}"
echo -e "  ${DIM}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
nl

# ── 1. System dependencies ─────────────────────────────────────────────────────
step "Checking system dependencies..."
PM="$(detect_pkg_manager)"

# Go
if ! command -v go &>/dev/null; then
  warn "Go not found."
  if [[ "$PM" == "unknown" ]]; then
    error "Could not detect a package manager — install Go manually: https://go.dev/dl/"
    exit 1
  fi
  if prompt_yn "Install Go now?" "y"; then
    case "$PM" in
      pacman) install_pkg pacman go ;;
      apt)    install_pkg apt golang-go ;;
      dnf)    install_pkg dnf golang ;;
      zypper) install_pkg zypper go ;;
    esac
    ok "Go installed: $(go version)"
  else
    error "Go is required to build — aborting."
    exit 1
  fi
else
  ok "Go found: $(go version | awk '{print $3}')"
fi

# hidapi (C library + dev headers required for cgo)
if hidapi_present; then
  ok "hidapi found"
else
  warn "hidapi not found (required for USB HID communication)."
  if [[ "$PM" == "unknown" ]]; then
    warn "Could not detect a package manager — install hidapi manually, then re-run."
    warn "  Arch:   sudo pacman -S hidapi"
    warn "  Debian: sudo apt install libhidapi-dev libhidapi-hidraw0"
    warn "  Fedora: sudo dnf install hidapi-devel"
    exit 1
  fi
  if prompt_yn "Install hidapi now?" "y"; then
    case "$PM" in
      pacman) install_pkg pacman hidapi ;;
      apt)    install_pkg apt libhidapi-dev libhidapi-hidraw0 ;;
      dnf)    install_pkg dnf hidapi hidapi-devel ;;
      zypper) install_pkg zypper hidapi-devel ;;
    esac
    ok "hidapi installed"
  else
    error "hidapi is required — aborting."
    exit 1
  fi
fi
nl

# ── 2. Build ───────────────────────────────────────────────────────────────────
step "Building ${BOLD}${BINARY}${NC}..."
if go build -o "${BINARY}" ./cmd/streamdeck/ 2>&1; then
  ok "Build complete"
else
  error "Build failed — aborting."
  exit 1
fi
nl

# ── 3. udev rule ───────────────────────────────────────────────────────────────
step "Checking udev rule..."
if [[ -f "${UDEV_RULE}" ]]; then
  ok "udev rule already installed — skipping"
else
  info "Installing udev rule (requires sudo):"
  dim "  ${UDEV_RULE}"
  echo 'KERNEL=="hidraw*", ATTRS{idVendor}=="0fd9", MODE="0666"' \
    | sudo tee "${UDEV_RULE}" >/dev/null
  sudo udevadm control --reload
  sudo udevadm trigger
  ok "udev rule installed — device accessible without root"
fi
nl

# ── 4. Binary ──────────────────────────────────────────────────────────────────
step "Installing binary to ${BOLD}${BIN_DIR}/${BINARY}${NC}..."
mkdir -p "${BIN_DIR}"
install -Dm755 "${BINARY}" "${BIN_DIR}/${BINARY}"
ok "Binary installed"
nl

# ── 5. Dotfiles ────────────────────────────────────────────────────────────────
echo -e "  ${BOLD}Config location${NC}"
echo -e "  ${DIM}─────────────────────────────────────────────${NC}"
nl
info "streamdeck-go stores its config and icons in a single directory."
info "You can keep that directory inside your dotfiles repo and symlink it"
info "into ~/.config — the same pattern used by Hyprland, Waybar, etc."
nl

USE_DOTFILES=false
CONFIG_DIR=""

if prompt_yn "Use a dotfiles directory?" "y"; then
  nl
  DOTFILES_RAW="$(prompt_input "Path to dotfiles repo" "${DEFAULT_DOTFILES}")"
  DOTFILES="$(abspath "${DOTFILES_RAW}")"

  if [[ ! -d "${DOTFILES}" ]]; then
    warn "Directory ${DOTFILES} does not exist — it will be created."
  fi

  CONFIG_DIR="${DOTFILES}/.config/streamdeck-go"
  SYMLINK_TARGET="${XDG_CONFIG}/streamdeck-go"

  nl
  echo -e "  ${DIM}─────────────────────────────────────────────${NC}"
  info "${BOLD}Will create:${NC}"
  dim "    ${CONFIG_DIR}/"
  dim "    ${CONFIG_DIR}/config.yaml  ${DIM}(if absent)${NC}"
  dim "    ${CONFIG_DIR}/icons/"
  nl
  info "${BOLD}Will symlink:${NC}"
  dim "    ${SYMLINK_TARGET}"
  dim "    └─▶ ${CONFIG_DIR}"
  echo -e "  ${DIM}─────────────────────────────────────────────${NC}"
  nl

  if ! prompt_yn "Confirm?" "y"; then
    nl
    warn "Aborted — nothing written."
    exit 0
  fi

  USE_DOTFILES=true
else
  nl
  CONFIG_DIR="${XDG_CONFIG}/streamdeck-go"
  info "Config will go directly in ${CONFIG_DIR}"
fi

nl

# ── 6. Create config directory ─────────────────────────────────────────────────
step "Setting up config directory..."
mkdir -p "${CONFIG_DIR}/icons"
ok "Created ${CONFIG_DIR}/icons/"

if [[ ! -f "${CONFIG_DIR}/config.yaml" ]]; then
  install -Dm644 config.example.yaml "${CONFIG_DIR}/config.yaml"
  ok "Default config written to config.yaml"
else
  ok "config.yaml already exists — not overwritten"
fi

# ── 7. Symlink (dotfiles mode only) ───────────────────────────────────────────
if [[ "${USE_DOTFILES}" == "true" ]]; then
  nl
  step "Creating symlink..."
  SYMLINK_TARGET="${XDG_CONFIG}/streamdeck-go"

  if [[ -L "${SYMLINK_TARGET}" ]]; then
    existing="$(readlink -f "${SYMLINK_TARGET}")"
    if [[ "${existing}" == "${CONFIG_DIR}" ]]; then
      ok "Symlink already correct — skipping"
    else
      warn "Symlink exists but points to ${existing}"
      if prompt_yn "Replace it?" "y"; then
        rm "${SYMLINK_TARGET}"
        ln -s "${CONFIG_DIR}" "${SYMLINK_TARGET}"
        ok "Symlink updated → ${CONFIG_DIR}"
      else
        warn "Symlink not updated — you may need to fix this manually."
      fi
    fi
  elif [[ -d "${SYMLINK_TARGET}" ]]; then
    warn "${SYMLINK_TARGET} is a real directory (not a symlink)."
    info "Move its contents to ${CONFIG_DIR} first, then re-run install."
    warn "Skipping symlink — config will work from ${CONFIG_DIR} but is not linked."
  else
    mkdir -p "${XDG_CONFIG}"
    ln -s "${CONFIG_DIR}" "${SYMLINK_TARGET}"
    ok "Symlinked ${SYMLINK_TARGET} → ${CONFIG_DIR}"
  fi
fi

# ── 8. Systemd user service ────────────────────────────────────────────────────
nl
step "Installing systemd user service..."
mkdir -p "${SYSTEMD_USER}"
install -Dm644 systemd/streamdeck-go.service "${SYSTEMD_USER}/streamdeck-go.service"
systemctl --user daemon-reload
systemctl --user enable --now streamdeck-go.service
ok "Service enabled and started"

# ── Done ───────────────────────────────────────────────────────────────────────
nl
echo -e "  ${DIM}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  ${GREEN}${BOLD}All done.${NC}"
nl

if [[ "${USE_DOTFILES}" == "true" ]]; then
  info "Your config lives in your dotfiles repo:"
  dim "    ${CONFIG_DIR}/config.yaml"
  dim "    ${CONFIG_DIR}/icons/"
else
  info "Your config:"
  dim "    ${CONFIG_DIR}/config.yaml"
  dim "    ${CONFIG_DIR}/icons/"
fi

nl
info "Edit and save config.yaml — the deck reloads automatically."
info "For privileged commands (suspend, reboot, etc): ${BOLD}make install-helper${NC}"
nl
info "Logs: ${DIM}journalctl --user -u streamdeck-go -f${NC}"
nl

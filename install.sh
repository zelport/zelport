#!/usr/bin/env bash
# Canonical source: https://github.com/zelport/zelport/blob/main/install.sh
# The website copy is synchronized by GitHub Actions.
# Install Zelport without changing its configuration or installing desktop plugins.
# Keep the release and reviewed Arch recipe pins together when publishing updates.

ZELPORT_INSTALLER_VERSION=0.1.2
ZELPORT_RELEASE=0.1.0-preview.5
ZELPORT_ARCH_REF=56128665770b6399ed5896272732ab7fd2b7e923
ZELPORT_ARCH_SHA256=4f48486805a10b1293c3574611fba3117b67bcf8b2e70c719745fc8b18fd5bfa

say() { printf '%s\n' "$*"; }
die() { printf 'Error: %s\n' "$*" >&2; exit 1; }
require() { command -v "$1" >/dev/null 2>&1 || die "Required command not found: $1"; }

cleanup() {
    local result=$?
    trap - EXIT
    # Go module-cache directories are read-only. Change only directories inside
    # our private build workspace; find does not follow symlinks by default.
    if ! find "$work_dir" -type d -exec chmod u+w {} + || ! rm -rf -- "$work_dir"; then
        printf 'Warning: could not fully remove temporary build directory: %s\n' "$work_dir" >&2
    fi
    if [[ -n ${install_tmp:-} ]] && ! rm -f -- "$install_tmp"; then
        printf 'Warning: could not remove temporary binary: %s\n' "$install_tmp" >&2
    fi
    exit "$result"
}

usage() {
    cat <<'EOF'
Usage: bash install.sh [--no-start] [--help]

macOS: display Homebrew instructions only.
Windows shells: display the upcoming-support notice only.
Arch / Omarchy: build and install the verified pacman package with makepkg.
Other Linux: install the verified release binary into ~/.local/bin.

By default, enable and start the user service when a systemd user manager is
available. Restart an active service only when its installation changes.
--no-start installs only; it does not configure, enable, start or restart services.
Without a systemd user manager, install only and print manual instructions.
Run as your regular user, never with sudo. Only makepkg's package installation
and dependency installation may request sudo. Desktop plugins are separate.
EOF
}

fetch() {
    curl --fail --location --silent --show-error --proto '=https' --proto-redir '=https' \
        --connect-timeout 20 --max-time 600 --retry 2 "$1" --output "$2"
}

digest() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | awk '{print $1}'
    else
        shasum -a 256 "$1" | awk '{print $1}'
    fi
}

verify() {
    [[ $2 =~ ^[0-9a-fA-F]{64}$ ]] || die 'Missing or invalid SHA-256 checksum.'
    [[ $(digest "$1") == "$2" ]] || die "Checksum mismatch: ${1##*/}. Nothing from this download was installed."
}

detect_distro() {
    # Read only identifiers; do not execute /etc/os-release as shell code.
    if [[ -r /etc/os-release ]]; then
        sed -n -e 's/^ID=//p' -e 's/^ID_LIKE=//p' /etc/os-release | tr -d "\"'" | tr '\n' ' '
    fi
}

user_unit() {
    cat <<'EOF'
# Managed by the Zelport installer.
[Unit]
Description=Zelport local HTTP reverse proxy
After=network.target

[Service]
Type=simple
ExecStart=%h/.local/bin/zelport
Restart=on-failure
RestartSec=5
UMask=0077

[Install]
WantedBy=default.target
EOF
}

install_binary() {
    local asset="zelport_${ZELPORT_RELEASE}_linux_${cpu}.tar.gz"
    local base="https://github.com/zelport/zelport/releases/download/v${ZELPORT_RELEASE}"
    local expected members notice
    fetch "$base/SHA256SUMS" "$work_dir/SHA256SUMS"
    expected=$(awk -v name="$asset" '$2 == name {print $1}' "$work_dir/SHA256SUMS")
    fetch "$base/$asset" "$work_dir/$asset"
    verify "$work_dir/$asset" "$expected"
    members=$(tar -tzf "$work_dir/$asset")
    [[ $(printf '%s\n' "$members" | awk '$0 == "zelport" {n++} END {print n+0}') == 1 ]] || die 'Archive must contain exactly one zelport binary.'
    # Stream only the binary to a chosen path, never extract archive paths to disk.
    tar -xOzf "$work_dir/$asset" zelport > "$work_dir/zelport"
    [[ -s $work_dir/zelport ]] || die 'The downloaded binary is empty.'
    for notice in LICENSE THIRD_PARTY_NOTICES.txt; do
        tar -xOzf "$work_dir/$asset" "$notice" > "$work_dir/$notice"
        [[ -s $work_dir/$notice ]] || die "Missing release notice: $notice"
    done
    mkdir -p "$data_dir"
    for notice in LICENSE THIRD_PARTY_NOTICES.txt; do
        [[ ! -L $data_dir/$notice ]] || die "Refusing to replace a symlink: $data_dir/$notice"
        install -m 644 "$work_dir/$notice" "$data_dir/$notice"
    done
    mkdir -p "$HOME/.local/bin"
    install_tmp=$(mktemp "$HOME/.local/bin/.zelport-install.XXXXXX")
    install -m 755 "$work_dir/zelport" "$install_tmp"
    mv -f -- "$install_tmp" "$binary"
    install_tmp=''
}

install_arch() {
    fetch "https://raw.githubusercontent.com/zelport/zelport/${ZELPORT_ARCH_REF}/contrib/archlinux/PKGBUILD" "$work_dir/PKGBUILD"
    verify "$work_dir/PKGBUILD" "$ZELPORT_ARCH_SHA256"
    local pkgver pkgrel installed
    pkgver=$(sed -n 's/^pkgver=//p' "$work_dir/PKGBUILD")
    pkgrel=$(sed -n 's/^pkgrel=//p' "$work_dir/PKGBUILD")
    [[ $pkgver =~ ^[a-zA-Z0-9.+_]+$ && $pkgrel =~ ^[0-9]+$ ]] || die 'Invalid pinned Arch package version.'
    installed=$(pacman -Q zelport 2>/dev/null || true)
    if [[ $installed == "zelport $pkgver-$pkgrel" ]] && pacman -Qk zelport >/dev/null 2>&1; then
        say "Zelport $pkgver-$pkgrel is already installed."
        return
    fi
    command -v makepkg >/dev/null 2>&1 || die 'Install Arch build tools first: sudo pacman -S --needed base-devel git'
    changed=yes
    say 'Building the Arch package. Package installation may request your sudo password.'
    # A pipe-fed installer must leave package-manager prompts on the terminal.
    if [[ ! -t 0 ]] && ( : < /dev/tty ) 2>/dev/null; then
        (cd "$work_dir" && makepkg --syncdeps --install < /dev/tty)
    else
        (cd "$work_dir" && makepkg --syncdeps --install)
    fi
}

start_service() {
    if [[ $changed == yes || $unit_changed == yes ]]; then systemctl --user daemon-reload; fi
    if [[ $was_active == yes && ( $changed == yes || $unit_changed == yes ) ]]; then
        systemctl --user restart zelport.service || die 'Installed, but the existing service could not restart. Inspect journalctl --user -u zelport.service.'
    elif [[ $was_active == no ]]; then
        if ! systemctl --user start zelport.service; then
            systemctl --user stop zelport.service || true
            die 'Installed, but startup failed. Inspect journalctl --user -u zelport.service. No other process was stopped.'
        fi
    fi
    sleep 1
    if ! systemctl --user is-active --quiet zelport.service; then
        if [[ $was_active == no ]]; then systemctl --user stop zelport.service || true; fi
        die 'Installed, but the service is not running. Check for a port conflict with journalctl --user -u zelport.service.'
    fi
    if ! systemctl --user is-enabled --quiet zelport.service; then
        systemctl --user enable zelport.service
    fi
    say 'Zelport is running and enabled for your user session. Dashboard: http://localhost:8080'
}

main() {
    set -euo pipefail
    say "Zelport installer $ZELPORT_INSTALLER_VERSION"
    local start=yes platform distro method existing unit_dir unit fragment
    local cpu binary data_dir was_active=no use_systemd=no changed=no unit_changed=no installed_version
    for arg in "$@"; do
        case "$arg" in
            --help|-h) usage; return 0 ;;
            --no-start) start=no ;;
            *) die "Unknown argument: $arg. Use --help." ;;
        esac
    done
    platform=$(uname -s)
    case "$platform" in
        Darwin)
            say 'On macOS, use Homebrew as your regular user:'
            say '  brew install zelport/tap/zelport'
            say '  brew services start zelport'
            return 0 ;;
        MINGW*|MSYS*|CYGWIN*|Windows_NT)
            say 'Windows support is coming soon. No changes were made.'
            return 0 ;;
        Linux) ;;
        *) die "Unsupported operating system: $platform" ;;
    esac
    [[ $(id -u) != 0 ]] || die 'Run this installer as your regular user, without sudo.'
    [[ ${HOME:-} == /* && $HOME != / ]] || die 'HOME must be an absolute user home directory.'
    case "$(uname -m)" in
        x86_64|amd64) cpu=amd64 ;;
        aarch64|arm64) cpu=arm64 ;;
        *) die 'This installer supports Linux x86_64 and aarch64 only.' ;;
    esac
    require curl
    require tar
    if ! command -v sha256sum >/dev/null 2>&1; then require shasum; fi
    distro=$(detect_distro)
    case " $distro " in
        *' arch '*|*' archlinux '*) method=arch; binary=/usr/bin/zelport ;;
        *) method=binary; binary="$HOME/.local/bin/zelport" ;;
    esac
    existing=$(command -v zelport || true)
    [[ -z $existing || $existing == "$binary" ]] || die "Zelport already exists at $existing. Update it with its original installer; refusing a second installation."
    if [[ $method == arch ]]; then
        require pacman
        [[ ! -e $HOME/.local/bin/zelport && ! -L $HOME/.local/bin/zelport ]] || die 'A user-local Zelport exists. Remove that installation before switching to pacman.'
        if [[ -e $binary ]]; then
            pacman -Q zelport >/dev/null 2>&1 || die 'An unmanaged /usr/bin/zelport exists; refusing to replace it.'
        fi
    else
        [[ ! -L $binary ]] || die 'The installation target is a symlink. Update Zelport using its original installer.'
        [[ ! -e $binary || -f $binary ]] || die 'The installation target is not a regular file.'
        data_dir="${XDG_DATA_HOME:-$HOME/.local/share}/zelport"
        [[ $data_dir == /* ]] || die 'XDG_DATA_HOME must be absolute when set.'
    fi
    if [[ $start == yes ]] && command -v systemctl >/dev/null 2>&1 && systemctl --user show-environment >/dev/null 2>&1; then
        use_systemd=yes
        if systemctl --user is-active --quiet zelport.service; then was_active=yes; fi
    fi
    # The temporary directory is removed even when verification or installation fails.
    work_dir=$(mktemp -d "${TMPDIR:-/tmp}/zelport-install.XXXXXX")
    install_tmp=''
    trap cleanup EXIT
    if [[ $use_systemd == yes ]]; then
        if [[ $method == binary ]]; then
            unit_dir="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
            [[ $unit_dir == /* ]] || die 'XDG_CONFIG_HOME must be absolute when set.'
            unit="$unit_dir/zelport.service"
            user_unit > "$work_dir/zelport.service"
            [[ ! -L $unit ]] || die 'Existing service is a symlink. Use --no-start and manage the service yourself.'
            if [[ -e $unit ]] && ! cmp -s "$unit" "$work_dir/zelport.service"; then
                die 'Existing custom user service was preserved. Use --no-start and manage the service yourself.'
            fi
            fragment=$(systemctl --user show -p FragmentPath --value zelport.service 2>/dev/null || true)
            [[ -z $fragment || $fragment == "$unit" ]] || die 'A service from another installation exists. Use its original installation method.'
        fi
        if [[ $was_active == no ]] && command -v ss >/dev/null 2>&1; then
            if [[ -n $(ss -H -ltn 'sport = :8080') ]]; then
                die 'Port 8080 is already in use. Stop the conflicting instance yourself or use --no-start. No process was stopped.'
            fi
        fi
    fi
    say "Checking Zelport $ZELPORT_RELEASE ($method, $cpu)."
    if [[ $method == arch ]]; then
        install_arch
    else
        installed_version=''
        if [[ -x $binary ]]; then installed_version=$("$binary" --version 2>/dev/null || true); fi
        if [[ $installed_version == "zelport $ZELPORT_RELEASE (built "*")" ]]; then
            say "Zelport $ZELPORT_RELEASE is already installed."
        else
            install_binary
            changed=yes
        fi
    fi
    if [[ $use_systemd == yes ]]; then
        if [[ $method == binary ]]; then
            mkdir -p "$unit_dir"
            if [[ ! -f $unit ]]; then
                install -m 644 "$work_dir/zelport.service" "$unit"
                unit_changed=yes
            fi
        fi
        start_service
    else
        printf 'Installed. Start Zelport manually: %q\n' "$binary"
        if [[ $start == yes ]]; then say 'No reachable systemd user manager. Automatic startup was not configured.'; fi
    fi
    if [[ $method == binary ]]; then
        case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) say 'Add ~/.local/bin to PATH to use the zelport command.' ;; esac
    fi
    say 'The Omarchy plugin is separate: omarchy plugin add https://github.com/zelport/zelport-omarchy.git --enable'
}

# Sourcing exposes functions for isolated tests; execution and curl | bash run main.
if [[ ${BASH_SOURCE[0]:-} == "$0" || -z ${BASH_SOURCE[0]:-} ]]; then main "$@"; fi

#!/usr/bin/env bash
set -euo pipefail

# atb-cli installer
# Usage: curl -fsSL https://raw.githubusercontent.com/AMR-genomics-hackathon-2026/atb-cli-claude/main/install.sh | bash

REPO="AMR-genomics-hackathon-2026/atb-cli-claude"
BINARY="atb"
INSTALL_DIR="${ATB_INSTALL_DIR:-/usr/local/bin}"

info()  { printf "\033[1;34m==>\033[0m %s\n" "$1"; }
error() { printf "\033[1;31merror:\033[0m %s\n" "$1" >&2; exit 1; }

detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *) error "Unsupported OS: $(uname -s)" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *) error "Unsupported architecture: $(uname -m)" ;;
    esac
}

latest_version() {
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | tr ',' '\n' \
        | grep '"tag_name"' \
        | head -1 \
        | tr -d ' "' \
        | cut -d: -f2
}

main() {
    local os arch version url ext tmp

    os=$(detect_os)
    arch=$(detect_arch)

    info "Detected platform: ${os}/${arch}"

    if [ -n "${ATB_VERSION:-}" ]; then
        version="$ATB_VERSION"
        info "Using specified version: ${version}"
    else
        info "Fetching latest release..."
        version=$(latest_version)
        if [ -z "$version" ]; then
            error "Could not determine latest version. Set ATB_VERSION=vX.Y.Z to install a specific version."
        fi
        info "Latest version: ${version}"
    fi

    # Strip leading 'v' for the archive filename
    local ver_num="${version#v}"

    if [ "$os" = "windows" ]; then
        ext="zip"
    else
        ext="tar.gz"
    fi

    url="https://github.com/${REPO}/releases/download/${version}/atb-cli_${ver_num}_${os}_${arch}.${ext}"

    info "Downloading ${url}"

    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT

    if ! curl -fsSL -o "${tmp}/archive.${ext}" "$url"; then
        error "Download failed. Check that version ${version} exists at:\n  https://github.com/${REPO}/releases"
    fi

    info "Extracting..."
    if [ "$ext" = "zip" ]; then
        unzip -q "${tmp}/archive.zip" -d "$tmp"
    else
        tar -xzf "${tmp}/archive.tar.gz" -C "$tmp"
    fi

    if [ ! -f "${tmp}/${BINARY}" ]; then
        error "Binary '${BINARY}' not found in archive"
    fi

    # Install
    if [ -w "$INSTALL_DIR" ]; then
        mv "${tmp}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    else
        info "Installing to ${INSTALL_DIR} (requires sudo)"
        sudo mv "${tmp}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    fi
    chmod +x "${INSTALL_DIR}/${BINARY}"

    info "Installed ${BINARY} ${version} to ${INSTALL_DIR}/${BINARY}"
    echo ""
    "${INSTALL_DIR}/${BINARY}" version 2>/dev/null || true
    echo ""
    info "Run 'atb fetch' to download the database, then 'atb query --help' to get started."
}

main "$@"

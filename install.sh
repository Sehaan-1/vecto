#!/bin/sh
# Vecto macOS & Linux Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/Sehaan-1/vecto/master/install.sh | bash

set -e

REPO="Sehaan-1/vecto"
VECTO_BIN_NAME="vecto"

# Output helpers
info() {
    printf "\033[36m[-] %s\033[0m\n" "$1"
}

success() {
    printf "\033[32m[✓] %s\033[0m\n" "$1"
}

warn() {
    printf "\033[33m[!] %s\033[0m\n" "$1"
}

error() {
    printf "\033[31m[x] %s\033[0m\n" "$1" >&2
    exit 1
}

# 1. Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
    linux) OS="linux" ;;
    darwin) OS="darwin" ;;
    *) error "Unsupported operating system: $OS. Vecto supports Linux and Darwin (macOS)." ;;
esac

# 2. Detect Architecture
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) error "Unsupported architecture: $ARCH. Vecto supports amd64 and arm64." ;;
esac

# 3. Determine Version
VERSION="${VECTO_VERSION:-}"
if [ -z "$VERSION" ]; then
    info "Fetching latest release information..."
    if command -v curl >/dev/null 2>&1; then
        VERSION="$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')"
    elif command -v wget >/dev/null 2>&1; then
        VERSION="$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')"
    else
        error "Neither curl nor wget is installed."
    fi
fi

if [ -z "$VERSION" ]; then
    error "Could not determine latest version from GitHub API."
fi

# Clean version for filename (strip leading v)
CLEAN_VERSION="${VERSION#v}"
TAG="v${CLEAN_VERSION}"

info "Installing Vecto ${TAG} for ${OS}/${ARCH}..."

ASSET_NAME="vecto_${CLEAN_VERSION}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/Sehaan-1/vecto/releases/download/${TAG}/${ASSET_NAME}"
CHECKSUMS_URL="https://github.com/Sehaan-1/vecto/releases/download/${TAG}/checksums.txt"

# Temporary directory
TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t 'vecto')"
cleanup() {
    rm -rf "$TMP_DIR"
}
trap cleanup EXIT

# 4. Download archive and checksums
info "Downloading ${DOWNLOAD_URL}..."
if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$DOWNLOAD_URL" -o "${TMP_DIR}/${ASSET_NAME}"
    curl -fsSL "$CHECKSUMS_URL" -o "${TMP_DIR}/checksums.txt"
else
    wget -qO "${TMP_DIR}/${ASSET_NAME}" "$DOWNLOAD_URL"
    wget -qO "${TMP_DIR}/checksums.txt" "$CHECKSUMS_URL"
fi

# 5. Checksum verification
info "Verifying SHA-256 checksum..."
EXPECTED_HASH="$(grep "${ASSET_NAME}" "${TMP_DIR}/checksums.txt" | awk '{print $1}')"

if [ -z "$EXPECTED_HASH" ]; then
    error "Asset ${ASSET_NAME} not found in checksums.txt"
fi

if command -v sha256sum >/dev/null 2>&1; then
    COMPUTED_HASH="$(sha256sum "${TMP_DIR}/${ASSET_NAME}" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
    COMPUTED_HASH="$(shasum -a 256 "${TMP_DIR}/${ASSET_NAME}" | awk '{print $1}')"
else
    warn "Neither sha256sum nor shasum found; skipping checksum verification."
    COMPUTED_HASH="$EXPECTED_HASH"
fi

if [ "$COMPUTED_HASH" != "$EXPECTED_HASH" ]; then
    error "Checksum verification failed! Expected: ${EXPECTED_HASH}, Computed: ${COMPUTED_HASH}"
fi
success "Checksum verified."

# 6. Extract archive
info "Extracting..."
tar -xzf "${TMP_DIR}/${ASSET_NAME}" -C "$TMP_DIR"

if [ ! -f "${TMP_DIR}/${VECTO_BIN_NAME}" ]; then
    error "Binary ${VECTO_BIN_NAME} not found inside downloaded archive."
fi

# 7. Install directory selection
INSTALL_DIR="${VECTO_INSTALL_DIR:-}"
if [ -z "$INSTALL_DIR" ]; then
    if [ -w "/usr/local/bin" ]; then
        INSTALL_DIR="/usr/local/bin"
    elif [ "$(id -u)" -eq 0 ]; then
        INSTALL_DIR="/usr/local/bin"
    else
        INSTALL_DIR="${HOME}/.local/bin"
        mkdir -p "$INSTALL_DIR"
    fi
fi

# Install binary
TARGET="${INSTALL_DIR}/${VECTO_BIN_NAME}"
if [ -w "$INSTALL_DIR" ]; then
    cp "${TMP_DIR}/${VECTO_BIN_NAME}" "$TARGET"
    chmod +x "$TARGET"
else
    info "Elevated permissions required to write to ${INSTALL_DIR}."
    sudo cp "${TMP_DIR}/${VECTO_BIN_NAME}" "$TARGET"
    sudo chmod +x "$TARGET"
fi

success "Installed ${VECTO_BIN_NAME} to ${TARGET}"

# 8. Check PATH
case ":$PATH:" in
    *":${INSTALL_DIR}:"*) ;;
    *)
        warn "${INSTALL_DIR} is not in your \$PATH."
        warn "Add it to your shell profile (e.g., ~/.bashrc or ~/.zshrc):"
        printf "  export PATH=\"%s:\$PATH\"\n\n" "$INSTALL_DIR"
        ;;
esac

# 9. Verify
if [ -x "$TARGET" ]; then
    "$TARGET" --version || true
fi

echo ""
success "Vecto ${TAG} successfully installed!"
info "Run 'vecto --help' or 'vecto init' to get started."

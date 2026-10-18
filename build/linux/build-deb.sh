#!/usr/bin/env bash
set -euo pipefail

# build-deb.sh - Builds the production binary and packages it into a Debian (.deb) package
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Ensure Node & Go toolchain paths
export PATH="${HOME}/.nvm/versions/node/$(ls -1 ${HOME}/.nvm/versions/node 2>/dev/null | tail -n 1)/bin:/usr/local/go/bin:${HOME}/go/bin:${PATH}"

VERSION="1.0.0"
ARCH="amd64"
DEB_NAME="netprobe-wifi_${VERSION}_${ARCH}.deb"
BIN_NAME="netprobe-wifi"
OUTPUT_DIR="${PROJECT_ROOT}/build/bin"
STAGING_DIR="$(mktemp -d /tmp/netprobe-deb-XXXXXX)"

cleanup() {
    rm -rf "${STAGING_DIR}"
}
trap cleanup EXIT

echo "=== 1. Building production binary with Wails ==="
cd "${PROJECT_ROOT}"
wails build -tags webkit2_41 -clean -o "${BIN_NAME}"

PROD_BIN="${OUTPUT_DIR}/${BIN_NAME}"
if [ ! -f "${PROD_BIN}" ]; then
    echo "Error: Binary not found at ${PROD_BIN}"
    exit 1
fi

echo "=== 2. Assembling Debian package structure ==="
mkdir -p "${STAGING_DIR}/DEBIAN"
mkdir -p "${STAGING_DIR}/usr/bin"
mkdir -p "${STAGING_DIR}/usr/share/applications"
mkdir -p "${STAGING_DIR}/usr/share/icons/hicolor/256x256/apps"

# Copy control and postinst
cp "${PROJECT_ROOT}/build/linux/DEBIAN/control" "${STAGING_DIR}/DEBIAN/"
cp "${PROJECT_ROOT}/build/linux/DEBIAN/postinst" "${STAGING_DIR}/DEBIAN/"
chmod 755 "${STAGING_DIR}/DEBIAN/postinst"

# Copy binary
cp "${PROD_BIN}" "${STAGING_DIR}/usr/bin/${BIN_NAME}"
chmod 755 "${STAGING_DIR}/usr/bin/${BIN_NAME}"

# Copy desktop launcher
cp "${PROJECT_ROOT}/build/linux/netprobe-wifi.desktop" "${STAGING_DIR}/usr/share/applications/"
chmod 644 "${STAGING_DIR}/usr/share/applications/netprobe-wifi.desktop"

# Copy icon
if [ -f "${PROJECT_ROOT}/build/linux/netprobe-wifi.png" ]; then
    cp "${PROJECT_ROOT}/build/linux/netprobe-wifi.png" "${STAGING_DIR}/usr/share/icons/hicolor/256x256/apps/netprobe-wifi.png"
    chmod 644 "${STAGING_DIR}/usr/share/icons/hicolor/256x256/apps/netprobe-wifi.png"
fi

echo "=== 3. Packaging .deb with dpkg-deb ==="
mkdir -p "${OUTPUT_DIR}"
dpkg-deb --build --root-owner-group "${STAGING_DIR}" "${OUTPUT_DIR}/${DEB_NAME}"

echo ""
echo "=== Package built successfully! ==="
echo "Location: ${OUTPUT_DIR}/${DEB_NAME}"
echo ""
echo "--- Package Metadata ---"
dpkg-deb -I "${OUTPUT_DIR}/${DEB_NAME}"
echo ""
echo "--- Package Contents ---"
dpkg-deb -c "${OUTPUT_DIR}/${DEB_NAME}"

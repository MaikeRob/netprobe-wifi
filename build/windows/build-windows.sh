#!/usr/bin/env bash
set -euo pipefail

# build-windows.sh - Builds the Windows executable and NSIS installer
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Ensure Node & Go toolchain paths
export PATH="${HOME}/.nvm/versions/node/$(ls -1 ${HOME}/.nvm/versions/node 2>/dev/null | tail -n 1)/bin:/usr/local/go/bin:${HOME}/go/bin:${PATH}"

cd "${PROJECT_ROOT}"

echo "=== Building NetProbe Wi-Fi for Windows (AMD64) with NSIS installer ==="
wails build -platform windows/amd64 -nsis -webview2 download -clean

echo ""
echo "=== Windows build complete! Check build/bin/ for netprobe-wifi.exe and installer ==="
ls -lh "${PROJECT_ROOT}/build/bin"

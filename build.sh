#!/usr/bin/env bash
set -euo pipefail

BIN_DIR="${HOME}/.local/bin"

# Build client
echo "[LESDFS] Building client..."
(cd client && go build -o lesdfs)

# Build daemon
echo "[LESDFS] Building daemon..."
(cd daemon && go build -o lesdfs-daemon)

echo -e "\033[92mBuild completed successfully.\033[0m"

# Optional installation
read -p "Do you want to install client and daemon to $BIN_DIR? (y/n): " ans
if [[ "$ans" =~ ^[Yy]$ ]]; then
    install -Dm755 client/lesdfs "$BIN_DIR/lesdfs"
    install -Dm755 daemon/lesdfs-daemon "$BIN_DIR/lesdfs-daemon"
    echo "Client and daemon installed to $BIN_DIR"
fi

# Optional cleanup of project binaries
read -p "Do you want to remove the built binaries from the project folders? (y/n): " clean_ans
if [[ "$clean_ans" =~ ^[Yy]$ ]]; then
    rm -f client/lesdfs daemon/lesdfs-daemon
    echo "Project binaries removed."
fi
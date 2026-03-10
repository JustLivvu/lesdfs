# LESDFS - Secure Encrypted Filesystem in Go

![License](https://img.shields.io/badge/license-MIT-blue)

**LESDFS** is a lightweight Go-based encrypted vault system. It provides secure, FUSE-backed virtual filesystems for storing sensitive data. Vaults are mounted and managed through a background daemon and accessed via a CLI client.
> [!WARNING]
> This project is still under development.
---

## Features

- Encrypted virtual filesystem using AES-GCM.
- Password-based key derivation with Argon2.
- FUSE-based mount for seamless filesystem integration.
- Background daemon managing vault mounts.
- Simple CLI for creating, opening, listing, and deleting vaults.
- Automatic unmounting on shutdown.

---

## Architecture

The project consists of three main components:

### 1. Daemon (`daemon/main.go`)
The daemon is responsible for:

- Mounting and unmounting vaults via HTTP endpoints.
- Running a FUSE filesystem for each vault.
- Managing active mounts and cleaning up on shutdown.

**HTTP API:**

- `GET /mount?vault=<name>&path=<path>&password=<password>` — Mount a vault.
- `GET /unmount?vault=<name>` — Unmount a vault.

The daemon listens on `127.0.0.1:8080`.

---

### 2. VaultFS (`vaultfs/vaultfs.go`)
VaultFS implements the FUSE filesystem for each vault:

- Files and directories are encrypted using AES-GCM.
- Directory structure mirrors the vault folder on disk.
- `.salt` file is used for secure key derivation with Argon2.

VaultFS provides standard filesystem operations:

- `Read`, `Write`, `Create`, `Remove`, `Mkdir`, `ReadDirAll`

---

### 3. CLI Client (`client/main.go`)
A lightweight command-line interface for interacting with the daemon and vaults.

**Supported commands:**

| Command | Description |
|---------|-------------|
| `--create <vault_name>` | Create a new encrypted vault |
| `--list` | List all existing vaults |
| `--mount <vault_name>` | Mount a vault via the daemon |
| `--umount <vault_name>` | Unmount a vault via the daemon |
| `--delete <vault_name>` | Delete a vault permanently |

**Example:**

```bash
# Create a new vault
lesdfs --create my_vault

# List all vaultsLesd
lesdfs --list

# Open and mount a vault
lesdfs --mount my_vault

# Close and unmount a vault
lesdfs --umount my_vault

# Delete a vault
lesdfs --delete my_vault
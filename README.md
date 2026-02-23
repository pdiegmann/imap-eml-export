# imap-eml-export

[![CI](https://github.com/pdiegmann/imap-eml-export/actions/workflows/ci.yml/badge.svg)](https://github.com/pdiegmann/imap-eml-export/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/pdiegmann/imap-eml-export)](https://github.com/pdiegmann/imap-eml-export/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

> Export every email from an IMAP server to local `.eml` files — and optionally import them back into a different server. Perfect for backups, migrations, and archiving. No installation required.

## Features

- **Export** every folder of any IMAP mailbox to `.eml` files, preserving the complete folder hierarchy
- **Import** those files back into a (different) IMAP server to migrate an entire mailbox
- **Gmail / Google Workspace** sign-in via OAuth2 — no app-password or special knowledge required
- Interactive **setup wizard** that guides you on the first run and saves your settings for next time
- Live **progress dashboard** with folder name, message counter, speed, and elapsed time
- **Config file** + **environment variables** + **CLI flags** with clear priority ordering
- **Self-update** — keeps itself up to date with one command

---

## Quick Start

**1. Download** the binary for your platform from the [latest release](https://github.com/pdiegmann/imap-eml-export/releases/latest).

**2. Make it executable** (macOS / Linux):

```bash
chmod +x imap-eml-export
```

**3. Run it:**

```bash
# Let the interactive wizard guide you
./imap-eml-export export

# Or supply everything on the command line
./imap-eml-export export \
    --export-host imap.example.com \
    --export-username me@example.com \
    --export-password secret \
    --output ./backup

# Gmail / Google Workspace — OAuth2 sign-in, no password needed
./imap-eml-export export --google --export-username me@gmail.com
```

**4. Done.** Your emails are in `./output/`, organised by folder.

---

## Commands

```
imap-eml-export <command> [flags]
```

| Command   | Description |
|-----------|-------------|
| `export`  | Download all emails from an IMAP server to local `.eml` files |
| `import`  | Upload `.eml` files from a local directory to an IMAP server |
| `update`  | Self-update to the latest release |
| `version` | Print the current version |

Run any command with `--help` to see its flags:

```bash
./imap-eml-export --help
./imap-eml-export export --help
./imap-eml-export import --help
```

---

## CLI Flags

### Global flags (all commands)

| Flag | Description | Default |
|------|-------------|---------|
| `--config <path>` | Config file path | `~/.config/imap-eml-export/config.toml` |
| `--log-file <path>` | Write logs to a file | |
| `-v`, `--verbose` | Verbose output | `false` |
| `--debug` | Debug output (very verbose) | `false` |

### `export` flags

| Flag | Description | Default |
|------|-------------|---------|
| `--export-host <host>` | IMAP hostname of the source server | |
| `--export-port <port>` | IMAP port of the source server | `993` |
| `-u`, `--export-username <user>` | Login username (usually your email address) | |
| `-p`, `--export-password <pass>` | Login password (use an App Password for Gmail) | |
| `-o`, `--output <dir>` | Directory where `.eml` files are written | `./output` |
| `--export-tls` | Use implicit TLS/IMAPS | `true` |
| `--export-starttls` | Upgrade to TLS via STARTTLS (use with `--export-tls=false`) | `false` |
| `--google` | Sign in with Google OAuth2 — sets host/port/TLS automatically | `false` |
| `-y`, `--yes` | Skip confirmation prompts | `false` |

### `import` flags

| Flag | Description | Default |
|------|-------------|---------|
| `--import-host <host>` | IMAP hostname of the target server | |
| `--import-port <port>` | IMAP port of the target server | `993` |
| `-u`, `--import-username <user>` | Login username for the target server | |
| `-p`, `--import-password <pass>` | Login password for the target server | |
| `-i`, `--input <dir>` | Directory of `.eml` files to upload | `import.input_dir` or `export.output_dir` from config |
| `--import-tls` | Use implicit TLS/IMAPS | `true` |
| `--import-starttls` | Upgrade to TLS via STARTTLS (use with `--import-tls=false`) | `false` |
| `--google` | Sign in with Google OAuth2 — sets host/port/TLS automatically | `false` |

---

## Gmail / Google Workspace

Standard Gmail and Google Workspace accounts block plain-password IMAP login. Use the `--google` flag (or `google = true` in the config) to authenticate via OAuth2 instead.

### What happens on first run

1. The tool prints a short URL and a code.
2. Open the URL in any browser and sign in with your Google account.
3. Enter the code when prompted.
4. The refresh token is cached locally — subsequent runs are fully automatic.

### Example

```bash
# Export from Gmail
./imap-eml-export export --google --export-username me@gmail.com

# Import into a Google Workspace account
./imap-eml-export import --google --import-username me@company.com --input ./backup
```

### Providing OAuth2 credentials

The tool needs a Google OAuth2 **Client ID** and **Client Secret**. Supply them via environment variables or in the config file (see below).

**How to obtain credentials** (one-time setup):

1. Go to the [Google Cloud Console](https://console.cloud.google.com/).
2. Create a project → **APIs & Services** → **Credentials**.
3. Click **Create credentials** → **OAuth 2.0 Client ID**.
4. Application type: **Desktop app**.
5. Copy the **Client ID** and **Client Secret**.

Set them as environment variables:

```bash
export GOOGLE_CLIENT_ID=YOUR_CLIENT_ID.apps.googleusercontent.com
export GOOGLE_CLIENT_SECRET=YOUR_CLIENT_SECRET
```

Or add them to the config file (see [Config File](#config-file) below).

---

## Environment Variables

All settings can be provided via environment variables. The prefix is `IMAP_` followed by the section (`EXPORT` or `IMPORT`) and the key name.

```bash
# Export source server
export IMAP_EXPORT_HOST=imap.example.com
export IMAP_EXPORT_PORT=993
export IMAP_EXPORT_USERNAME=me@example.com
export IMAP_EXPORT_PASSWORD=secret
export IMAP_EXPORT_OUTPUT_DIR=./backup
export IMAP_EXPORT_TLS=true

# Import target server
export IMAP_IMPORT_HOST=imap.newserver.com
export IMAP_IMPORT_PORT=993
export IMAP_IMPORT_USERNAME=me@newserver.com
export IMAP_IMPORT_PASSWORD=secret
export IMAP_IMPORT_INPUT_DIR=./backup

# Google OAuth2 credentials (shared by both export and import)
export GOOGLE_CLIENT_ID=YOUR_CLIENT_ID.apps.googleusercontent.com
export GOOGLE_CLIENT_SECRET=YOUR_CLIENT_SECRET
```

---

## Config File

Default location: `~/.config/imap-eml-export/config.toml`

The config file uses separate `[export]` and `[import]` sections so that both source and target accounts can live in a single file. See [`config.example.toml`](config.example.toml) for a fully annotated template.

**Priority:** CLI flags > environment variables > config file > defaults.

### Standard IMAP server

```toml
[export]
host       = "imap.example.com"
port       = 993
username   = "me@example.com"
# WARNING: password is stored in plaintext. Restrict permissions: chmod 600 ~/.config/imap-eml-export/config.toml
password   = "your-password"
output_dir = "./output"
tls        = true
starttls   = false

[import]
host      = "imap.newserver.com"
port      = 993
username  = "me@newserver.com"
password  = "your-password"
input_dir = "./output"
tls       = true
starttls  = false
```

### Gmail / Google Workspace via OAuth2

```toml
[export]
google   = true
username = "me@gmail.com"

[export.oauth2]
client_id     = "YOUR_CLIENT_ID.apps.googleusercontent.com"
client_secret = "YOUR_CLIENT_SECRET"
# refresh_token is populated automatically after the first sign-in

[import]
google   = true
username = "dest@company.com"

[import.oauth2]
client_id     = "YOUR_CLIENT_ID.apps.googleusercontent.com"
client_secret = "YOUR_CLIENT_SECRET"
```

---

## Output Structure

Exported emails are saved as individual `.eml` files under the output directory, mirroring the IMAP folder hierarchy:

```
output/
├── INBOX/
│   ├── 00001_2024-01-15_hello-world.eml
│   └── 00002_2024-01-16_another-subject.eml
├── Sent/
│   └── 00001_2024-01-10_re-proposal.eml
├── Work/
│   └── ProjectA/
│       └── 00001_2024-02-01_proposal.eml
└── Drafts/
```

File names follow the pattern `{sequence}_{YYYY-MM-DD}_{sanitized-subject}.eml`.

---

## Releasing a New Version

Use `scripts/release.sh` to bump the version and create a signed git tag:

```bash
# Bump the patch version (e.g. v0.1.1 → v0.1.2) — this is the default
./scripts/release.sh

# Bump the minor version (e.g. v0.1.1 → v0.2.0)
./scripts/release.sh minor

# Bump the major version (e.g. v0.1.1 → v1.0.0)
./scripts/release.sh major
```

The script reads the latest `v*.*.*` tag, increments the chosen component (resetting lower ones to zero), asks for confirmation, then creates an annotated tag and pushes it.

---

## Self-Update

```bash
./imap-eml-export update
```

Downloads and replaces the current binary with the latest GitHub release.

---

## Building from Source

Requires Go 1.24+.

```bash
git clone https://github.com/pdiegmann/imap-eml-export.git
cd imap-eml-export

# Quick build
go build -o imap-eml-export ./cmd/imap-eml-export

# With version embedded
go build -ldflags "-X main.version=v1.0.0" -o imap-eml-export ./cmd/imap-eml-export

# Build all platforms locally (uses scripts/build-local.sh)
bash scripts/build-local.sh v1.0.0
```

---

## Local IMAP Test Server

A Docker Compose setup is included to spin up a local Dovecot IMAP server pre-loaded with sample emails. This lets you run the exporter end-to-end and compare the output against the known input.

**Requirements:** Docker with Compose plugin (or `docker-compose` v2).

### Start the server

```bash
docker compose up -d
```

| Port | Protocol | Notes |
|------|----------|-------|
| 143  | plain IMAP | used by `config.test.toml` |
| 993  | IMAPS (TLS) | self-signed certificate |

Credentials: `testuser` / `testpassword`

Pre-loaded folders:

| Folder | Messages |
|--------|----------|
| `INBOX` | 3 |
| `Sent` | 2 |
| `Work` | 1 |
| `Work/ProjectA` | 1 |

### Run against the test server

```bash
./imap-eml-export export --config config.test.toml -y
```

Exported files land in `./test-output/`. Compare them with the sample sources in `dev/imap/sample-emails/`.

### Stop the server

```bash
docker compose down
```

---

## Running Tests

```bash
go test ./...
# or
bash scripts/test.sh
```

---

## Contributing

1. Fork the repo and create a feature branch.
2. Make your changes and add tests.
3. Run `go test ./...` and `go vet ./...`.
4. Open a pull request.

---

## License

[MIT](LICENSE) © pdiegmann

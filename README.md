# imap-eml-export

[![CI](https://github.com/pdiegmann/imap-eml-export/actions/workflows/ci.yml/badge.svg)](https://github.com/pdiegmann/imap-eml-export/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/pdiegmann/imap-eml-export)](https://github.com/pdiegmann/imap-eml-export/releases/latest)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

> Export all emails from an IMAP server to local `.eml` files, mirroring the server's folder hierarchy — no installation required.

## Why?

Standard email clients make it cumbersome to export a full mailbox. `imap-eml-export` is a single executable that connects directly to any IMAP server and downloads every message in every folder, preserving the complete directory structure. It's perfect for backups, migrations, and archiving.

---

## Quick Start

**1. Download** the binary for your platform from the [latest release](https://github.com/pdiegmann/imap-eml-export/releases/latest).

**2. Run it** (on macOS/Linux, `chmod +x` first):

```bash
./imap-eml-export export
```

A TUI wizard will guide you through the setup on first run, then save your settings for future use.

**3. Done.** Your emails are in the `./output/` directory, organised by folder.

---

## Usage

### Subcommands

| Command | Description |
|---------|-------------|
| `export` | Export emails from IMAP (default if no subcommand given) |
| `update` | Self-update to the latest release |
| `version` | Print the current version |

### CLI Flags

**Global flags** (available on all subcommands):

| Flag | Description |
|------|-------------|
| `--config <path>` | Config file path (default: `~/.config/imap-eml-export/config.toml`) |
| `--log-file <path>` | Write logs to a file |
| `-v`, `--verbose` | Enable verbose output |
| `--debug` | Enable debug output |

**`export` flags:**

| Flag | Description | Default |
|------|-------------|---------|
| `--host <host>` | IMAP server hostname | |
| `--port <port>` | IMAP server port | `993` |
| `-u`, `--username <user>` | IMAP username | |
| `-p`, `--password <pass>` | IMAP password | |
| `-o`, `--output <dir>` | Output directory | `./output` |
| `--tls` | Use implicit TLS | `true` |
| `--starttls` | Use STARTTLS upgrade | `false` |
| `-y`, `--yes` | Skip confirmations | `false` |

### Environment Variables

All settings can be overridden with `IMAP_`-prefixed environment variables:

```bash
export IMAP_HOST=imap.example.com
export IMAP_PORT=993
export IMAP_USERNAME=user@example.com
export IMAP_PASSWORD=secret
export IMAP_OUTPUT_DIR=./backup
export IMAP_TLS=true
```

### Config File

The config file is TOML. Default location: `~/.config/imap-eml-export/config.toml`.

See [`config.example.toml`](config.example.toml) for a fully commented example:

```toml
host       = "imap.gmail.com"
port       = 993
username   = "your-email@gmail.com"
# WARNING: password is stored in plaintext. Restrict file permissions: chmod 600.
password   = "your-app-password"
output_dir = "./output"
tls        = true
starttls   = false
```

**Priority order:** CLI flags > environment variables > config file > defaults.

### Output Structure

```
output/
├── INBOX/
│   ├── 00001_2024-01-15_hello-world.eml
│   └── 00002_2024-01-16_another-subject.eml
├── INBOX/Projects/
│   └── ClientA/
│       └── 00001_2024-02-01_proposal.eml
├── Sent/
└── Drafts/
```

File names follow the pattern `{sequence}_{date}_{sanitized-subject}.eml`.

---

## Self-Update

Check for and apply updates:

```bash
# Interactive (asks before downloading)
./imap-eml-export update

# Non-interactive
./imap-eml-export update --yes
```

---

## Building from Source

Requires Go 1.24+.

```bash
git clone https://github.com/pdiegmann/imap-eml-export.git
cd imap-eml-export
go build -o imap-eml-export ./cmd/imap-eml-export
```

With version embedded:

```bash
go build -ldflags "-X main.version=v1.0.0" -o imap-eml-export ./cmd/imap-eml-export
```

Build all platforms locally:

```bash
bash scripts/build-local.sh v1.0.0
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

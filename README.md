# go-cli-tool (Go)

A small, fast CLI tool to safely manage shell aliases, exports, and sudoers entries.
Written in Go — single static binary, quick startup, cross-platform (macOS + Linux).

## Features

- `alias` add/list/remove
- `export` add/list/remove
- `sudoers` add/list/remove/list — validated with `visudo`
- `backup` and `restore` for RC file and sudoers
- Environment overrides for safe testing:
  - `BASM_RC_FILE`
  - `BASM_SUDOERS_PATH`
  - `BASM_BACKUP_DIR`

## Build

```bash
# standard build
make build

# cross compile for macOS arm64 and linux amd64
make cross

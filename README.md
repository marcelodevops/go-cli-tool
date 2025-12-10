# shctl

`shctl` — tiny, fast CLI to safely manage shell aliases, exports, and sudoers entries.

## Features
- alias add/list/remove
- export add/list/remove
- sudoers add/list/remove (validated with `visudo`)
- backup & restore
- Safe testing via env overrides:
  - `BASM_RC_FILE` — rc file path
  - `BASM_SUDOERS_PATH` — sudoers path
  - `BASM_BACKUP_DIR` — backup directory

## Build
```bash
make build
# or cross:
make cross

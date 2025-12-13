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
```
## Usage
```bash
./shctl alias add ll "ls -la"
./shctl alias list
./shctl sudoers add "myuser ALL=(ALL) NOPASSWD: /usr/bin/somebinary"

```
## Testing (no root)
Set environment variables to temporary paths before running tests:
```bash
export BASM_RC_FILE=/tmp/test_rc
export BASM_SUDOERS_PATH=/tmp/test_sudoers
export BASM_BACKUP_DIR=/tmp/test_backups

make test
```
## Release
Use goreleaser (optional). Adjust goreleaser.yml with your repo owner/name and run:
```bash
goreleaser release --rm-dist

```

## Notes

- The tool validates sudoers changes via visudo -c -f <file> before applying.
- When applying to /etc/sudoers the tool uses sudo cp, so you will be prompted for your password.


---

## What I did **not** change
- I intentionally avoided adding external dependencies (Cobra, urfave/cli) to keep the binary tiny and avoid forcing vendoring/version bumps.
- `util` uses straightforward functions to keep things explicit and testable.

---


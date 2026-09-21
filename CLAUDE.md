# isoshelf

Go app (Windows, Linux) that inventories, update-checks, downloads and verifies the bootable images in a folder: a Ventoy USB drive, a NAS share, Proxmox ISO storage. A web page on 127.0.0.1 plus a CLI; MIT. The maintainer is new to Go and Git: explain any manual step plainly.

## Layout
- `cmd/isoshelf`: entry point and CLI (`scan`, `check`, `ui`)
- `internal/catalog`: catalog format and `default.toml`, the list of known images
- `internal/{source,resolve,verify,fetch,update}`: latest version → exact file and checksum → download → place
- `internal/{scan,sniff,state,inventory,check,identify}`: read a folder, keep its records, work out statuses
- `internal/web`: HTTP server and the embedded page. `static/index.html` loads
  one script per part of the page: `app.js` (state, asking, drawing, wiring),
  `images.js`, `details.js`, `downloads.js`, `actions.js`, `folders.js`,
  `archive.js`, `settings.js`, plus `app.css`
- `internal/remote/remotetest/recorded`: recorded HTTP responses the tests replay
- `docs/design.md` (full rules and design), `docs/STATUS.md` (where things stand), `docs/TODO.md` (what's next - read this first in a new session), `docs/catalog-sources.md`

## Run and test
- `go build ./... && go vet ./... && go test ./...` must pass before a commit (CI also checks `gofmt -l .`)
- `go run ./cmd/isoshelf ui --port 8765 --no-browser <folder>`: preview; restart after editing static files
- `go run ./internal/remote/remotetest/record -sizes`: the only thing that goes online; run after catalog changes

## Hard rules (details in docs/design.md)
- Only image files inside the folder the user picked; never partitions, bootloaders or `ventoy/`.
- Nothing is deleted unless the user chose it; archiving to `.isoshelf/removed` is always offered.
- A checksum mismatch blocks placement; an unverified download never replaces anything.
- Checksums only from the project's own HTTPS site. Never host or mirror images, automate vendor download flows (Windows is a link), or circumvent anything.
- Tests never touch the live network or real disks.

## Working rules
- Read only the files needed for the task; never read whole directories.
- Keep source files under ~200 lines; split by responsibility when they grow (a few older ones are still over: split them when you next change them).
- After finishing a task, update docs/STATUS.md with 3-5 lines (what changed, what's next) and tick the item off docs/TODO.md, which is what the next session picks up from.
- Delegate to the agent that fits (`.claude/agents/`):
  - `explorer` - find where something lives or how it works (read-only, haiku)
  - `worker` - small, clearly specified edits: renames, boilerplate, formatting
  - `page` - the web page: layout, styling, cards, dialogs, its scripts
  - `words` - UI text, errors, README, docs, changelog entries
  - `bugs` - reproduce and diagnose a "this doesn't work", fix it when small
  - `core` - the Go that reads and reasons: scan, check, catalog, queue, sizes
  Do yourself: architecture, anything that writes to or deletes from a drive
  (`internal/{update,fetch,verify}`, state writing), and any decision about
  what isoshelf should do.
- Delegating is not always cheaper. An agent starts with none of this
  session's context and has to work it out again, so a one-line fix costs more
  through an agent than done directly. Send work out when it is genuinely
  separable and more than a few minutes of reading: a sweep across many files,
  a bug that needs reproducing from scratch, a batch of wording. Keep a small
  edit in hand.
- Each batch of changes gets a version (0.0.1 steps) and a CHANGELOG.md section (the release build needs it). Catalog changes also raise its `revision` and get a dated CATALOG-CHANGES.md section.
- Commit and push after each step; keep README and docs current.
- Never stop or restart the maintainer's preview (port 8765) without asking; test downloads in a scratch folder.

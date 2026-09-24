# isoshelf

Go app (Windows, Linux) that inventories, update-checks, downloads and verifies the bootable images in a folder: a Ventoy USB drive, a NAS share, Proxmox ISO storage. A web page on 127.0.0.1 (or on a network, behind a login) plus a CLI; MIT. The maintainer is new to Go and Git: explain any manual step plainly.

## Layout
- `cmd/isoshelf`: entry point and CLI (`ui`, `scan`, `check`, `password`, `version`, `help`)
- `internal/catalog`: catalog format and `default.toml`, the list of known images
- `internal/{source,resolve,verify,fetch,update}`: latest version → exact file and checksum → download → place
- `internal/{scan,sniff,state,inventory,check,identify}`: read a folder, keep its records, work out statuses
- `internal/upload`: places a file dragged onto the page or picked from the user's own computer
- `internal/{catupdate,usercat}`: the catalog refreshing itself from this repository; images the user named themselves
- `internal/{settings,appdir,drives,space,lastcheck,version}`: this computer's choices; where isoshelf keeps its own files; drive names; room left; the last answer from each project; comparing versions
- `internal/{auth,peer}`: the username and password on a network; copying an image from another isoshelf
- `internal/appupdate`: isoshelf's own releases: the update notice, the signature check, and swapping in the new program
- `internal/web`: HTTP server and the embedded page. `static/index.html` loads
  one script per part of the page: `app.js` (state, asking, drawing, wiring),
  `images.js`, `summary.js` (the line saying what wants doing), `details.js`,
  `checklist.js`, `downloads.js`, `actions.js`, `catalog.js` (Add images),
  `identify.js` (What is this?), `missing.js` (getting a missing image back),
  `origin.js` (where a file came from, and Check it),
  `folders.js`,
  `archive.js`, `settings.js` (the panel), `settinglist.js` (what each
  setting is), `access.js` (sign-in and sharing), `records.js` (where a
  folder's records live), `upload.js`, `report.js`, `selfupdate.js`
  (isoshelf updating its own program), plus `app.css`
- `internal/remote/remotetest/recorded`: recorded HTTP responses the tests replay
- `internal/sampledrive`: real filenames used as test fixtures; its `mkdrive` command writes them to a real folder for trying the page against a full drive
- `internal/docs`: no code, just the test that keeps this repository's own claims about itself true
- `docs/design.md` (full rules and design), `docs/STATUS.md` (where things stand), `docs/TODO.md` (what's next - read this first in a new session), `docs/archive.md` (finished work moved out of those two), `docs/catalog-sources.md`

## Run and test
- `go build ./... && go vet ./... && go test ./...` must pass before a commit (CI also checks `gofmt -l .`)
- `go run ./cmd/isoshelf ui --port 8765 --no-browser <folder>`: preview; restart after editing static files
- `go run ./internal/remote/remotetest/record -sizes`: the only thing that goes online; run after catalog changes

## Branches and releases
`main` is what the latest release was built from: it always matches something
people have downloaded. Work never happens directly on it.

- **One branch per piece of work, named after the work**: `settings`,
  `auto-check`, `fix-filter-menu`. Not a date, not a session id, not
  `next-version`. If you can't name it, the piece of work is too vague.
- Branch from the current `main`, commit as you go, push, and open a pull
  request when it's done. Merge it, then delete the branch - it has served
  its purpose and its commits live on in `main`.
- **Releasing** is pushing a `v*` tag on `main`: the workflow builds the
  three binaries and the portable zip, names them with the version, attaches
  `SHA256SUMS`, and takes the release notes from that version's
  `CHANGELOG.md` section. No section, no release - the workflow stops.
- **Only a tag with a suffix is a pre-release** (`v1.0.0-rc1`). Marking every
  `v0.*` as one was tried in v0.4.10 and taken back out in v0.4.11: every
  isoshelf already installed asks GitHub for `/releases/latest`, which skips
  pre-releases, so marking them stopped anybody being told a new version
  existed - and the fix shipped in the release they would first have had to
  be told about. **Before changing what a release is labeled, work out what
  the copies already installed will ask for.** `internal/appupdate` reads the
  list rather than "the latest", which is the more robust thing anyway and is
  what makes a real `-rc` release work later.
- The maintainer is new to Git, so say which of these you are doing and why,
  in plain words, rather than just running it.
- **In the sandbox these sessions run in, the catalog cannot be worked on.**
  The egress proxy answers 403 to CONNECT for almost every host the catalog
  depends on - `endoflife.date`, `cdimage.debian.org`, and every wish-list
  project's own site that was tried - and `api.github.com` is scoped to the
  repositories attached to the session, so a `type = "github"` source is
  refused whatever token is set. `releases.ubuntu.com` answers, so it is an
  allowlist rather than a blanket block. This is why the weekly catalog
  routine produces nothing: it fires, runs, and has no way to reach the
  projects it is meant to ask. **The live check belongs in GitHub Actions**
  (`.github/workflows/catalog-check.yml`, `workflow_dispatch`), which has
  real network and a real token; read its result and write the catalog from
  that, rather than trying to fetch anything here.
- **In the sandbox these sessions run in**, the git proxy refuses `v*` tag
  pushes and branch deletions with a 403. So a release is started by hand
  from the Actions tab with the version typed in, and a merged branch that
  GitHub didn't delete itself has to be deleted on the website. The clone is
  shallow with no tags fetched, so `git tag` prints nothing even when
  releases exist - ask GitHub, not the clone. None of this is true of the
  project itself, so it stays out of the documents people read.

## Hard rules (details in docs/design.md)
- Only image files inside the folder the user picked; never partitions, bootloaders or `ventoy/`.
- Nothing is deleted unless the user chose it; archiving to `.isoshelf/removed` is always offered.
- A checksum mismatch blocks placement; an unverified download never replaces anything.
- Checksums only from the project's own HTTPS site. Never host or mirror images, automate vendor download flows (Windows is a link), or circumvent anything.
- Tests never touch the live network or real disks.

## Working rules
- Read only the files needed for the task; never read whole directories.
- Keep source files under ~200 lines; split by responsibility when they grow (a few older ones are still over: split them when you next change them).
- After finishing a task, update docs/STATUS.md with 3-5 lines (what changed, what's next) and move the finished item from docs/TODO.md to docs/archive.md; TODO.md is what the next session picks up from.
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
- **The README's Roadmap block is the thing that rots.** "Next", "Also in
  v0.5" and "Later" are checked by nobody and by no test: a feature named
  there stays named after it ships. It went eleven releases listing work that
  was already done (server mode, where records live), and the maintainer found
  it, not me. Re-read those four lines at every release, whether or not the
  release touched anything they mention.
- **Nothing in the repository may say something that is no longer true, and
  this is checked before every push, not later.** People read this repository
  on GitHub without cloning it, so a stale sentence is the product as far as
  they are concerned. Before pushing, re-read whatever the change affects -
  README, `docs/*.md`, `CLAUDE.md`, issue templates, workflow text - and make
  it agree with what the code now does: version numbers, counts (catalog
  entries, sample-drive files, script names), file names, and anything
  described as "next" or "not done yet". `CHANGELOG.md` and
  `CATALOG-CHANGES.md` are the exception: they are a record of what each
  release held, so old versions stay written there exactly as they were.
- **Better still, don't write down anything that has to be maintained.** A
  download is "the file ending in `-windows-amd64.exe`", never a file name
  with a real version in it; an example version is written `vMAJOR.MINOR.PATCH`
  or `vX.Y.Z`. A test (`internal/docs`) fails the build if a release file name
  carrying a real version reappears outside the changelogs.
- Commit and push after each step; keep README and docs current.
- Never stop or restart the maintainer's preview (port 8765) without asking; test downloads in a scratch folder.

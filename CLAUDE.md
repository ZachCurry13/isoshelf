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
  one script per part of the page: `icons.js` (Octicons, and Escape hiding a tooltip), `jumpnav.js` (the tab bar), `app.js` (state, asking, drawing, wiring),
  `images.js`, `logotile.js`, `menus.js`, `imagerow.js` (one image), `blankslate.js`, `summary.js` (the line saying what wants doing), `details.js`,
  `checklist.js`, `downloads.js` (the queue), `dockitems.js`, `dockprogress.js`
  (the downloads bar), `notices.js`, `actions.js`, `catalog.js` (Add images),
  `identify.js` (What is this?), `missing.js` (getting a missing image back),
  `origin.js` (where a file came from, and Check it), `fromserver.js`
  (copying from your server), `dismiss.js` (dismissing an update), `older.js` (installing an older version, and the hold), `bootable.js` (Make bootable: renaming or unpacking a file the boot menu won't list),
  `duplicates.js` (copies of one image),
  `folders.js`, `drivelist.js` (saving a folder's list, and filling a folder from one), `thisfolder.js` (the This folder group in Settings),
  `archive.js`, `settings.js` (the panel), `settinglist.js`, `updatesettings.js` (what each
  setting is), `access.js` (sign-in and sharing), `records.js` (where a
  folder's records live), `upload.js`, `report.js`, `selfupdate.js`
  (isoshelf updating its own program), `history.js` (what arrived, was
  updated and left), `system.js` (where isoshelf runs), plus `css/` (the stylesheets, one file per part of the page; `css/tokens.css` is the look)
- `internal/web/primer`: builds `static/css/primer.css` from Primer's published design tokens (`go run ./internal/web/primer/fetch`); the look is documented in `docs/design-system.md`, and `static/styleguide.html` is the page that shows all of it
- `internal/remote/remotetest/recorded`: recorded HTTP responses the tests replay
- `internal/sampledrive`: real filenames used as test fixtures; its `mkdrive` command writes them to a real folder for trying the page against a full drive
- `internal/docs`: no code, just the test that keeps this repository's own claims about itself true
- `docs/design.md` (full rules and design), `docs/design-system.md` (the look: tokens, stylesheets, how to change them), `docs/primer-guidelines.md` and `docs/primer-audit.md` (Primer's manual as a checklist, and the page checked against it), `docs/STATUS.md` (where things stand), `docs/TODO.md` (what's next - read this first in a new session), `docs/archive.md` (finished work moved out of those two), `docs/HANDOFF.md` (the 2026-09-29 handoff to a model on the maintainer's own computer), `docs/catalog-sources.md`

## Run and test
- `go build ./... && go vet ./... && go test ./...` must pass before a commit (CI also checks `gofmt -l .`)
- `go run ./cmd/isoshelf ui --port 8765 --no-browser <folder>`: preview; restart after editing static files
- `go run ./internal/remote/remotetest/record -sizes`: the only thing that goes online while checking the catalog (the logo and Primer fetchers download assets too); run after catalog changes
- `go run ./internal/web/primer/fetch`: rebuilds `css/primer.css` after changing `primer.Version` or the Primer names in `css/tokens.css`

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
  job moved on 2026-10-26 from a cloud routine (paused) to a weekly scheduled
  task on the maintainer's computer, working in its own checkout (a second
  clone of this repository, kept apart from the one where other work
  happens). **The live check belongs in GitHub Actions**
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

## Maintenance (adapted 2026-10-26 from the maintainer's protocol)
- What can be updated on its own: the Go libraries (`go-toml`, `xz` for
  Make bootable, `go-crypto` for OpenPGP and what it brings: `circl`,
  `x/crypto`, `x/sys`), the GitHub Actions versions, the container base image, and
  the catalog (live from `main`, no release needed). There are no other
  modules. Go is named only
  by its line (`go 1.27` in go.mod, `golang:1.27-alpine`), and every build
  takes the newest patch; moving to 1.28 is a decision, made by hand.
- Checking: `ci.yml` runs `govulncheck` on every change. Every Monday
  `audit.yml` scans the code and the newest released program and keeps one
  issue labeled `audit` (closed when clear): a hole only the released
  program has means a release is the whole fix. Dependabot proposes library,
  Actions and base image updates as pull requests. By hand:
  `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`, or
  `-mode=binary <file>` for a released program.
- **Claude applies by itself only a patch or minor update that fixes a known
  hole, on a branch, and never merges or releases**: that is the
  maintainer's call. **One exception, decided 2026-11-03: the weekly catalog
  task reviews and merges its own catalog pull request** when it passes the
  checklist in its instructions (checks green, only catalog files changed,
  nothing left for a decision, checksums on the project's own domain, no
  field a released isoshelf can't read); otherwise it leaves it open with a
  comment saying which item held it back. It never tags, releases, or merges
  anything else. Everything else (a major version, a base image or Go
  line jump, anything that changes behaviour) is reported in plain words:
  what changed and why it matters. One whose tests fail is closed, not
  forced. **Never "roll back" with `git checkout` or
  `git reset` over uncommitted work**: throw away the update's own branch.
- When reviewing GitHub: open issues and pull requests, including ones from
  other people (#84 was the first). An outside pull request is read for the
  hard rules below and for races and path handling before it is merged, and
  gets a CHANGELOG entry naming its author in the next release.

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
- **The look lives in `css/tokens.css`.** A rule uses isoshelf's names from there and never a colour, a shadow or a Primer name of its own; tests fail if it does (`internal/web/css_test.go`). Change the look by changing values there, and read `docs/design-system.md` first. `static/styleguide.html` shows every token and component in every theme: look at a change there.
- **The page follows Primer's manual, all of it, not only its colours** (the maintainer, 2026-11-04: "what's the point of having a design guide if you don't follow it"). `docs/primer-guidelines.md` is the manual as a checklist and `docs/primer-audit.md` is the page checked against it. A new or changed piece of the page uses Primer's component for the job (Banner, Blankslate, Label, Counter, UnderlineNav, Box and rows, ActionMenu or SelectPanel, ToggleSwitch, Dialog), Primer's spacing scale (`--sp-N`), Octicons, and Primer's content rules (sentence case, no punctuation on buttons, errors that say how to fix it). Check it against the checklist and `static/styleguide.html`, and against the chosen draft's look, before calling it done; add anything new to the style guide.
- Commit and push after each step; keep README and docs current.
- **Two computers work on this repository.** Before starting, and again after a
  few hours away, `git fetch` and read `docs/TODO.md` and `docs/STATUS.md` as
  they are on `main`: the other computer may have moved on. What is on a branch
  and not merged yet is said in `docs/STATUS.md`, so neither computer has to
  guess.
- Never stop or restart the maintainer's preview (port 8765) without asking; test downloads in a scratch folder.

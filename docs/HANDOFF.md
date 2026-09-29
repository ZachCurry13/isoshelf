# Handoff, 2026-09-29: to a model running on the maintainer's own computer

A snapshot, written when the work moved from cloud sessions to a model on
the maintainer's machine. It picks up from `docs/TODO.md` and
`docs/STATUS.md` as they stood at v0.8.5, says what happened since, and what
is different about working locally. It is dated on purpose: once its
"what next" has been done, trust `TODO.md` over this file.

## Read these first, in this order

1. **`CLAUDE.md`**: the rules. The hard rules (what may be written or
   deleted, checksums, never mirroring) are not negotiable. The part headed
   "In the sandbox these sessions run in" does **not** apply to you; see
   *Working locally* below.
2. **`docs/TODO.md`**: what to do next, in order.
3. **`docs/STATUS.md`**: where things stand, and the maintainer's decisions.
4. **`docs/design.md`**: the why, when a change touches something it covers.

`docs/archive.md` holds finished work and what was learned doing it. Read
the entry for anything you are about to change again.

## Where things stand

- **Released:** v0.8.7, published 2026-09-29, with the Windows program, both
  Linux programs, the portable zip, `SHA256SUMS` and its signature. The
  container image for v0.8.7 is published too. v0.8.6 was never released
  on its own; v0.8.7 carries it, and its notes say so.
- **`main`** matches that release, plus nothing else of substance. CI is
  green on it. No pull requests are open.
- **Two old branches** are left on GitHub: `claude/review-todo-docs-hri8gc`
  (from v0.3.5) and `missing-and-more` (from v0.6.1). Their work shipped
  long ago; ask the maintainer before deleting them.

## What happened since the last handoff

| Pull request | Version | What |
|---|---|---|
| #61 | none | The documents brought into line with the code; README made easier to read. |
| #63 | none | Finished work moved out of `TODO.md` and `STATUS.md` into `docs/archive.md`. |
| #72 | v0.8.6 | **Usage**: a folded section with the last 8 weeks of the folder and the facts about this isoshelf (`internal/state/usage.go`, `internal/web/usage.go`, `static/usage.js`). |
| #73 | v0.8.7 | **A crash fixed**: a page refresh during a scan could stop isoshelf with "concurrent map iteration and map write". |
| #74 | v0.8.7 | The v0.8.7 notes say they carry v0.8.6. |

Between v0.8.0 and v0.8.5, other sessions shipped origins (where each file
came from), copying anything from your server, dismissing an update, where
to find hand-fetched images, duplicates, and `isoshelf update` on the
command line. `CHANGELOG.md` has each one.

## What to do next

From `docs/TODO.md`, *Right now*:

1. **Item 8: AtlasOS recognized, not listed**, and **item 9: logos only
   where each project's own trademark policy allows them.** Item 8 needs a
   new catalog field, and the catalog refuses unknown fields
   (`DisallowUnknownFields`), so the field has to ship in one release before
   `default.toml` uses it. Otherwise every copy already installed refuses
   the updated catalog. Item 5's notes are waiting for the same reason; put
   them in together.
2. **[#6]: the Fedora entries stop pinning a release number.** When Fedora
   45 ships, isoshelf keeps offering 44 and nothing fails.
3. Then *After those* in `TODO.md`: #4 (two downloads at once, politely),
   #2, #3, #5, #11, #12, and #7 whenever a real USB drive is to hand.

Items 8, 9 and #6 need the projects' own websites. The cloud sessions
couldn't reach them; you can (see below).

## Working locally: what is different

The cloud sessions were boxed in. On the maintainer's machine:

- **The network works.** `go run ./internal/remote/remotetest/record -sizes`
  can reach every catalog source, so catalog work can happen here. It is
  still the only thing that goes online; tests never do.
- **Tags can be pushed**, so a release is `git tag vX.Y.Z` on `main` and then
  `git push origin vX.Y.Z`. That starts **both** the release workflow and
  the container image workflow by itself. Starting them by hand from the
  Actions tab, as the cloud sessions had to, also still works.
- **Branches can be deleted** after their pull request merges:
  `git push origin --delete <branch>`, then `git branch -d <branch>`.
- **The clone is not shallow**, so `git tag` lists the releases.
- **Port 8765 is the maintainer's own preview.** Never stop or restart it
  without asking. Try things on another port, such as 8799, against a
  scratch folder (`go run ./internal/sampledrive/mkdrive <folder>` makes
  one).
- You may not have `CLAUDE.md`'s helper agents (`explorer`, `worker`, and
  so on). Do that work yourself; the rules still apply.
- You may not have GitHub tools either. If `gh` is installed, it does pull
  requests (`gh pr create`, `gh pr merge --squash`); if not, push the branch
  and give the maintainer the link GitHub prints.

## Every change, in order

1. Branch from an up-to-date `main`, named after the work (`fedora-latest`,
   not a date): `git switch main && git pull && git switch -c <name>`.
2. Make the change. Keep source files under about 200 lines; split by
   responsibility when one grows.
3. Check: `go build ./... && go vet ./... && go test ./...`, and
   `gofmt -l .` must print nothing. If you touched anything a scan, a
   download or the server shares between goroutines, run
   `go test -race ./internal/web/ ./internal/inventory/ ./internal/state/`
   as well. CI runs `-race`.
4. Give it a version: the next `0.0.1` step. Then:
   - a `CHANGELOG.md` section, `## [vX.Y.Z] - YYYY-MM-DD`, written for people
   - `tag:` in `deploy/truenas/ix_values.yaml` and `app_version:` in
     `deploy/truenas/app.yaml` (a test fails if they don't match)
   - catalog changes also raise `revision` in `internal/catalog/default.toml`
     and get a dated `CATALOG-CHANGES.md` section
5. Docs:
   - `STATUS.md`: a new *Latest change*, with the previous one moved into
     `docs/archive.md`
   - the finished item: out of `TODO.md` and into `docs/archive.md`, with
     what was learned
   - the README's Roadmap block: reread its "Next", "Also planned" and
     "Later" at every release
6. Commit, push, open a pull request, wait for CI, merge (squash), delete the
   branch.
7. **Release every version.** A release's notes are that one version's
   changelog section, so a skipped version's news never reaches anyone; it
   happened with v0.3.1, v0.3.2 and v0.8.6. Tag, push the tag, and check the
   release page has its six files.

## Things that will catch you out

- **The `internal/docs` tests check the documents against the code.** Any
  document that lists one page script must list them all. The README's
  "N so far" must equal the number of catalog entries. No document outside
  the changelogs and `docs/archive.md` may name a release file with a real
  version in it: write "the file ending in `-windows-amd64.exe`". The tools
  list in the README must match the page's.
- **A new page script** goes in `internal/web/static/index.html`, in
  `scripts` in `internal/web/static_test.go`, and in the lists in
  `CLAUDE.md`, `docs/design.md` and `docs/TODO.md`.
- **The page's content policy forbids inline `style` attributes.** Set
  `element.style.width` from script instead, as `usage.js` does for its bars.
- **Anything handed to the server while a scan still runs must be a copy.**
  That was the v0.8.7 crash: `inventory.Run` gave its working state to
  `Interim`. `TestInterimIsTheCallersOwnCopy` guards it.
- **A catalog field ships before the catalog uses it** (see item 8 above).
- **`settings.Settings` holds maps, so a copy of it isn't a copy.** Work
  anything out from the old answer before setting the new one.
- **Before changing what a release is labeled, work out what installed
  copies will ask for.** Only a tag with a suffix, like `v1.0.0-rc1`, is a
  pre-release.

## The maintainer

New to Go and Git. Say what you're about to do in plain words before doing
it, especially anything with Git, and explain any step they have to take
themselves. They decide what isoshelf should do. Build what they've decided,
and ask when more than one answer is good.

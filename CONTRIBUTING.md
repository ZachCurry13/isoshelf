# Contributing

Thanks for looking. isoshelf is early, so most things are still easy to change.

## The most useful thing you can do

**Tell me about an image isoshelf doesn't know**, or gets wrong. Use the
[catalog form](https://github.com/ZachCurry13/isoshelf/issues/new?template=missing-image.yml),
or press "What is this file?" in isoshelf itself and use the link there — it
fills in most of the form for you.

The one detail that decides whether an image can be downloaded or only listed
is **where the project publishes its checksums**. If you know that address,
say so; it saves me the hunting.

Questions and "would isoshelf ever do X?" belong in
[Discussions](https://github.com/ZachCurry13/isoshelf/discussions). Issues are
for work that needs doing.

## Adding a catalog entry yourself

Entries live in [`internal/catalog/default.toml`](internal/catalog/default.toml),
one `[[entry]]` per track — a single product, edition, architecture and
channel. Why each existing entry is set up the way it is:
[`docs/catalog-sources.md`](docs/catalog-sources.md).

A minimal entry that isoshelf can download and verify:

```toml
[[entry]]
id = "example-desktop"
name = "Example Desktop"
arch = "x86_64"
match = 'example-(?P<version>[\d.]+)-desktop-amd64\.iso'
samples = ["example-24.04.1-desktop-amd64.iso"]
category = "desktop"
site = "https://example.org"
[entry.source]
type = "listing"
url = "https://example.org/releases/"
regex = 'Example (?P<version>[\d.]+)'
[entry.artifact]
base = "https://example.org/releases/{version}/"
file = 'example-{version}-desktop-amd64\.iso'
manifest = "SHA256SUMS"
```

The rules that trip people up:

- **Every URL must be the final address.** If it redirects, use where it lands.
  Tests can't replay redirects, and a checksum file that redirects to another
  host is refused at runtime.
- **Checksums come from the project's own HTTPS site.** Mirrors are fine for
  image bytes (they're checked against that checksum), never for the checksum.
- **One entry per track.** A 32-bit image must never be able to "update" to a
  64-bit one, and an LTS track never jumps to a non-LTS release.
- **Each sample filename must match its own entry and no other.** The validator
  checks this across the whole catalog.
- **No entry.artifact?** That's fine — the entry then reports versions and
  end-of-life but can't download, and needs a `page` to send people to.

Check your work:

```bash
go test ./internal/catalog/
```

That runs the validator, which reports every problem at once with line numbers.
Then check it against the live sites and re-record the responses the tests
replay:

```bash
go run ./internal/remote/remotetest/record
```

That is the only thing in this repository that goes online. It resolves every
entry, lists the ones that fail, and prints a note for every redirect.

## Building and testing

Go 1.27 or newer, nothing else. No cgo, no Node, no build step for the web UI.

```bash
go build ./...
go vet ./...
go test ./...
gofmt -l .
```

All four have to be clean; CI runs the same on Windows and Linux.

To try the web UI while you work on it:

```bash
go run ./cmd/isoshelf ui --port 8765 --no-browser /path/to/your/isos
```

Open the link it prints. The page, CSS and script are embedded in the binary,
so restart it after changing them.

Tests never touch a real drive (temp folders only) and never reach the network
(recorded responses only). Please keep it that way.

## Changing the code

[`docs/design.md`](docs/design.md) is the design document: the hard rules, how the four
stages fit together, and why things are the way they are. Read the hard rules
before changing anything that deletes, replaces or downloads a file. If a
change makes one of them wrong, say so in the pull request — the rule can
change, but not by accident.

Some things I won't merge, so you don't waste your time:

- Anything that mirrors or re-serves other people's images.
- Automating a vendor's download flow that isn't meant to be automated, or
  anything touching product keys or activation.
- Working around a rate limit, a CAPTCHA, or a paywall.
- Placing a file that failed its checksum, for any reason.

Text people read should sound like a person wrote it: plain words, no jargon
where an ordinary word exists, and error messages that say what to do next.
Small commits with messages that explain why, not what.

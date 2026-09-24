# Security

isoshelf downloads files from the internet and puts them in a folder you boot
from, so it is worth knowing how it tries not to hurt you, and how to tell me
when it does.

## Reporting something

Use GitHub's private reporting: **Security → Report a vulnerability** on
[this repository](https://github.com/ZachCurry13/isoshelf/security/advisories/new).
That reaches me without the report being public first.

Please don't open a public issue for anything that could be used against
someone before it's fixed. Anything else — a crash, a wrong version, a broken
download — is an ordinary issue.

I'm one person doing this in my spare time. I'll reply as soon as I can, and
I'd rather hear about something small than not hear about it.

## What isoshelf promises

These are the rules the code is written against. A report that one of them is
broken is a security report, not a feature request.

- **A checksum mismatch always blocks the file.** A download that doesn't match
  the checksum the project published is deleted, not placed.
- **Checksums come from the project's own HTTPS site, never a mirror.** Image
  bytes may come from a mirror, because those bytes are checked against a
  checksum fetched from the origin. A checksum file that redirects to another
  host is refused.
- **Nothing is replaced before the new file is verified.** The order is
  download, verify, rename into place, and only then deal with the old file —
  and only if you chose replacing.
- **A download nobody can verify never replaces anything by itself.**
- **isoshelf only writes inside the folder you pick** (plus its own settings
  folder, and its own program's folder when you update it), and only
  deletes image files there. Removing one asks you first,
  file by file, and always offers to archive it instead. An update follows
  the choice that image already carries — replace, archive or keep both,
  shown in its panel and changeable at any time — rather than asking again.
- **The web UI answers your computer only.** It listens on 127.0.0.1, needs a
  random token issued at startup, checks the `Host` header so a hostile website
  can't reach it by DNS rebinding, requires a custom header and a same-origin
  `Origin` for anything that changes state, and never inserts text from your
  drive as HTML.
- **The catalog it downloads is validated before use.** A catalog that doesn't
  parse, has an unknown key, a pattern that won't compile, or a schema from the
  future is refused, and the previous one is kept.
- **isoshelf installs a new version of itself only if the release carries
  this project's signature.** Each release's `SHA256SUMS` is signed with the
  project's Ed25519 key, whose public half is built into isoshelf, and every
  program in the update must match its line there. A missing or wrong
  signature blocks the update exactly as a checksum mismatch blocks an image.
  Nothing is downloaded until you press **Update now**; image downloads
  finish first; the old program is set aside, not deleted, until the new one
  has started, and put back if it doesn't. In a container isoshelf never
  replaces itself - pull the new image instead.
- **No telemetry.** isoshelf talks to the sites in the catalog, to GitHub for
  release information, and nowhere else.

## What isoshelf does not promise

- **That an image is what its project says it is.** isoshelf checks that the
  bytes match the checksum the project published. If a project's own site is
  compromised, a matching checksum proves nothing. Checking images' own
  signatures is planned and not done yet; isoshelf's updates of itself are
  signed (see above).
- **That an image is safe to boot.** It's an operating system image; isoshelf
  only manages the file.
- **Anything about a catalog you or someone else wrote by hand.** Entries in
  your own catalog file are used as written.

## Versions

Only the newest release is supported: security fixes go into a new release,
not into older ones. isoshelf tells you when a newer version is out, and since
v0.6.0 installs it itself when you press Update now.

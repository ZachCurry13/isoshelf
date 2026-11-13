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
- **A download nobody can verify never replaces anything by itself.** Nor
  does one that only an MD5 or SHA-1 vouches for: those still catch a
  damaged download, but they can't show a file is the one the project made.
- **A file is only called checked when it was.** Each file's records say
  where it came from and, when its hash matched a checksum the project
  publishes, which checksum and when. Nothing else marks a file checked —
  not a copy from another isoshelf, not the name it has, not a guess.
- **isoshelf only writes inside the folder you pick** (plus its own settings
  folder, and its own program's folder when you update it), and only
  deletes image files there. Removing one asks you first,
  file by file, and always offers to archive it instead. An update does with the
  old file what Settings says — replace, archive or keep both — rather than
  asking again, and never touches a file you pinned.
- **On your own computer, the web UI answers your computer only.** It listens
  on 127.0.0.1, needs a random token issued at startup, checks the `Host`
  header so a hostile website can't reach it by DNS rebinding, requires a
  custom header and a same-origin `Origin` for anything that changes state,
  and never inserts text from your drive as HTML.
- **On a server, nothing but a bare "ok" health check and the login form
  answers until you sign in.** Run in a container or on a network address,
  isoshelf takes a username and password. A test sends every route the
  server has, with every method, to a server that has a login and gets no
  answer but a refusal; another checks forged, altered and expired sessions.
  Until a login is set, the only way in is the link in the log, with a random
  secret on the end, and the form that sets one is for whoever has it: a
  stranger who finds the address gets a refusal, not the chance to choose the
  password (before v0.9.3 the first visitor could). Set `ISOSHELF_USERNAME` and
  `ISOSHELF_PASSWORD` and there is never a time without a login; the link
  stops working as soon as one exists. Passwords are kept as PBKDF2-SHA256
  hashes, changing one signs every other browser out, and the same checks on
  changes apply. It is one login, not user accounts.
- **Behind a tunnel or a proxy, the cookies are marked Secure** when the
  visitor is on https, so a browser that is ever sent to the `http://` address
  doesn't hand its session over in the clear; and, if you tell isoshelf it is
  behind a proxy it can trust (`ISOSHELF_TRUST_PROXY`), a stranger's wrong
  guesses make only the stranger wait, not you.
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
  replaces itself — pull the new image instead.
- **Sharing images with another isoshelf is off unless you turn it on.**
  When it's on, whoever can sign in can copy the images in the folder. An
  isoshelf copying from another one still checks every file against the
  project's own published checksum, so a wrong copy is thrown away, never
  placed.
- **No telemetry.** isoshelf talks to the sites in the catalog, to GitHub for
  release information and its own list of images, to another isoshelf on
  your network if you point it at one, and nowhere else.

## What isoshelf does not promise

- **That an image is what its project says it is.** isoshelf checks that the
  bytes match the checksum the project published. If a project's own site is
  compromised, a matching checksum proves nothing. isoshelf can check a
  project's OpenPGP signature on its checksum file (or image) and refuses a
  download whose signature fails (v0.14.0), but the catalog doesn't name any
  project's key yet, so today no image is checked that way. isoshelf's
  updates of itself are signed (see above).
- **That an image is safe to boot.** It's an operating system image; isoshelf
  only manages the file.
- **Anything about a catalog you or someone else wrote by hand.** Entries in
  your own catalog file are used as written.

## Versions

Only the newest release is supported: security fixes go into a new release,
not into older ones. isoshelf tells you when a newer version is out, and since
v0.6.0 installs it itself when you press Update now.

Every change is checked against Go's list of known security holes in Go itself
and in the libraries isoshelf uses, counting only the ones its code can
reach. Every Monday the newest release is checked the same way, as the
program people actually have, and anything found goes into an open issue
labeled `audit`. Releases are built with the newest Go patch, so when a hole
is in Go itself, a new release is the fix.

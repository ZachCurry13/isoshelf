# Changelog

What changed in each release of isoshelf, newest first.

Version numbers: the middle number rises for new abilities or a new look
(v0.3.0 was the redesign, v0.3.1 Settings, v0.3.2 checking by itself); the
last number rises for improvements to what it already does, like v0.3.3.

## [v0.4.14] - 2026-09-23

### Added
- **Downloads say where they are coming from.** While one runs, the bar now
  reads "... from the NAS" or "... from the internet", and the finished list
  says "Added from the NAS". Copying an image from another isoshelf on your
  network instead of fetching it again over the wire arrived in v0.4.9, and
  until now it was invisible while it happened: the same "Downloading 40%"
  whether the bytes were crossing the room or an ocean. It is also how you
  find out that a peer you set up is never actually being used, which short
  of watching a router was not findable at all.

## [v0.4.13] - 2026-09-22

### Changed
- **The words on the page.** A pass over every label, status, button and hint
  the page shows, for the ordinary word rather than a homemade one. Statuses
  now read *Update available*, *End of life*, *Download manually*,
  *Unrecognized*, *Checksum mismatch*, *Check failed* and *Won't boot from
  here*; each still explains itself when you hover over it, and the key under
  the list is unchanged. In Settings, *Check for updates by itself* is now
  *Check for updates automatically*, *Update the images by itself* is *Update
  images automatically*, *Who can get in* is *Sign-in*, and *This isoshelf*
  is *Version*.
- "Room" is called space everywhere, a failed download says *Failed* rather
  than *Didn't work*, and verifying a download is called verifying rather
  than checking, which is what the update check is called.

### Added
- **The browser tab says which isoshelf it is.** Anything reachable from
  another machine - a NAS, a container - is now "isoshelf server" in the tab
  and on its sign-in page. Somebody running one on their desktop and one on
  their NAS had two tabs called the same thing, and the one that can change a
  whole household's drive is the one worth being able to pick out.

### Fixed
- **The Roadmap in the README was eleven releases out of date.** It still
  listed server mode as "later" and where each folder's records live as
  "next", both of which shipped in v0.4.0 and v0.4.2. It now says what is
  actually next. A test counts the catalog for it, since "86 so far" is
  exactly the sort of number nobody re-reads.
- **The "By hand only" button could not be pressed.** When every update had
  to be done by hand, the updates card showed a greyed-out button reading
  *By hand only* - a label pretending to be a button. It is now a *Show them*
  button that filters the list down to those images, which is what anyone
  pressing it wanted.

## [v0.4.12] - 2026-09-22

### Fixed
- **Sign out works.** It answered `{"error": "request refused"}` and left you
  signed in. Signing out was a plain form posted to the login address, and
  isoshelf sends a header that makes browsers report the sender of a plain
  form as "null" - so isoshelf's own check for "did this come from me?"
  refused every sign-out there has ever been. It goes through the same door
  as every other button now.
  - The login form itself was never affected: it has its own way round that,
    because it has to work before you're signed in.

## [v0.4.11] - 2026-09-22

### Fixed
- **Updates work again.** v0.4.10 marked every release below 1.0 as a
  pre-release. Every isoshelf already installed asks GitHub for its *latest
  release*, and GitHub leaves pre-releases out of that - so once the newest
  release was marked, that question kept returning the last unmarked one and
  no isoshelf was ever told a new version existed. The fix for it shipped in
  the very release nobody could be told about.
  - Only a tag with a suffix - `v1.0.0-rc1` - is a pre-release now. Before
    1.0 the newest release **is** the release, and it has to be findable by
    the thing every installed copy asks.
  - What being before 1.0 means is said in the README and in the release
    notes instead, where it reaches people and breaks nothing.
  - Sorry. That one was avoidable: the trap was spotted for new code and
    missed entirely for the copies already out there.

## [v0.4.10] - 2026-09-22

### Changed
- **Every release below 1.0 is now marked a pre-release**, which is what it
  is. GitHub shows the badge and stops calling it the latest release. 1.0
  will be the point at which isoshelf does what it says and won't change
  under you; until then, saying so plainly is more use than a version number
  that implies otherwise.
- **isoshelf reads the list of releases rather than "the latest"**, because
  GitHub's idea of the latest release leaves pre-releases out entirely -
  which would have meant marking them quietly stopped anybody being told a
  new isoshelf exists. A pre-release is only offered to somebody already
  running one, which below 1.0 is everybody.

## [v0.4.9] - 2026-09-22

### Added
- **Copy an image from another isoshelf on your network instead of the
  internet.** If the isoshelf on your NAS already holds the file, the one on
  your laptop takes it from there - a minute over your own network instead of
  an hour over the wire. **Settings → Copy from another isoshelf first**: give
  it the address and the login for it.
  - **Nothing about verification changes, which is what makes it safe to
    prefer.** The checksum still comes from the project's own HTTPS site,
    never from the machine sending the bytes, and the bytes are still checked
    against it before anything is placed. A copy that turns out to be wrong -
    stale, damaged, or served by something pretending to be an isoshelf -
    costs one fall back to the real source and nothing else.
  - Anything the other isoshelf doesn't have comes from the internet as
    before. A peer that is switched off, asleep or unreachable doesn't stop a
    download; the page says so and it goes the long way round.
- **Let other isoshelfs copy from this one** - the other half, for the machine
  that has the images. Off unless you turn it on: sharing hands whole images
  to anyone who can sign in, which is a different thing from letting them
  manage the folder. With it on, scans hash every image rather than only those
  whose filename never changes, because a hash is how another isoshelf asks
  for one particular file rather than a name.
- **It remembers which drives it has served**, and what each one took. Worth
  having on its own, and it is most of what rebuilding a lost drive will need.

### Fixed
- **A download whose bytes don't match now tries the next place** rather than
  giving up. Every place is still checked just as hard, and bytes that match
  nothing anywhere still fail loudly - but a bad copy on one mirror no longer
  costs you the download.

## [v0.4.8] - 2026-09-22

### Fixed
- **Settings now says when something saves.** Every answer in there saves
  itself the moment you change it, which is the right behaviour and looks
  exactly like nothing happening. A green **Saved** now appears beside the
  setting you changed - beside that one, not somewhere general, because
  "saved" only reassures if you can tell what was - and clears itself after a
  few seconds.

### Changed
- **Settings reads shorter.** Each one now says what it does in a sentence or
  two. The warnings and small print haven't gone; they've moved to the line
  under the control, where they're read at the moment they matter instead of
  standing between you and the switch. The longest hint was 463 characters;
  none is now over 210, and a test keeps it that way.

## [v0.4.7] - 2026-09-22

### Changed
- **The link's secret stops working once you set a password.** A login now
  replaces it rather than sitting beside it: two ways in is two ways in, and
  the weaker of the two was the one printed in a log. Before anyone has set a
  password the link still works, because otherwise a new install couldn't be
  opened at all; after that it is nothing, and isoshelf stops printing it.
  - **Forgotten it?** Set `ISOSHELF_USERNAME` and `ISOSHELF_PASSWORD` and
    restart, or run **`isoshelf password`** on the machine isoshelf runs on -
    in a container, `docker exec -it isoshelf isoshelf password`. Both need
    the machine itself, which is the right bar for getting back in, and
    neither is a second door standing open on the network.
  - Nothing changes on your own computer, where there is no login and the
    link is how isoshelf opens itself.

### Added
- **`isoshelf password`** sets the username and password from a shell,
  keeping the username you already have.

### Fixed
- **Downloads failing with "permission denied" are now explained at startup,
  instead of one image at a time.** `.isoshelf` - isoshelf's own folder inside
  your images folder, where downloads are staged - is often older than the
  current arrangement: made on an earlier run by whichever user isoshelf was
  then. Change the app's user afterwards and that one folder is left behind,
  owned by the old one, while everything around it is fine. isoshelf checked
  the images folder and not that folder inside it, so nothing warned about it
  and every download failed.
  - It now says which folder, **who owns it**, how it got that way, and the
    exact `chown` to fix it - including that changing the app's user to match
    is the wrong answer here, because it would break the folder that works.

## [v0.4.6] - 2026-09-22

### Added
- **isoshelf can update the images by itself.** Turn it on in Settings, under
  *Checking for updates*, and every day - or every week - it checks, downloads
  every update it finds, verifies it against the project's published checksum,
  and puts it in place. Nothing is pressed.
  - **Each image still follows the answer it already carries** about its old
    copy: replace, archive, or keep both. What happens while you aren't
    watching is exactly what the page has been telling you would happen.
  - **It stops before filling the folder**, leaving room to spare, rather than
    updating a drive until nothing else fits on it.
  - Every rule that held before still holds: a checksum that doesn't match
    blocks the file, an unverified download never replaces anything, and
    nothing is deleted that you didn't already choose to lose.
  - A folder that is already being scanned or downloaded into is left alone
    until next time, and the page says when isoshelf started downloads by
    itself, so finding four of them underway is never a mystery.
  - **It is off unless you turn it on, and always will be.** This is the one
    thing isoshelf does that changes a drive while its owner isn't looking.
  - On a NAS it runs whenever the container is up. On your own computer it
    runs while isoshelf is open, and not when it's closed.

### Fixed
- **Two things saving settings at once could lose one of them.** Now that
  isoshelf writes to that file on a schedule as well as when you press
  something, every change to it takes a turn - so a switch you flick can't be
  quietly undone by isoshelf noting the time.

## [v0.4.5] - 2026-09-22

### Added
- **A username and password.** isoshelf on a network now asks you to choose
  one the first time you open it, and every device signs in with them after
  that. A sign-in lasts a month and survives a restart, so updating the app
  doesn't sign you out. Settings, under *Who can get in*, changes them, signs
  this browser out, or signs every browser out at once.
  - Set `ISOSHELF_USERNAME` and `ISOSHELF_PASSWORD` before it ever starts -
    in a container's environment variables - and there is no first-run
    question at all. Without them, whoever opens the address first is the one
    who chooses, and isoshelf says so plainly in its log rather than leaving
    it to be discovered.
  - **The link with the secret still works**, and is the way back in if you
    forget the password. Asked to change the password, isoshelf doesn't ask
    for the old one when you arrived by the link - which is the whole point
    of it.
  - The password itself is never stored, only a scrambled form of it
    (PBKDF2-HMAC-SHA256, 600,000 rounds, its own random salt). After a few
    wrong guesses from one address, that address is made to wait.
  - On your own computer nothing changes: nobody outside the machine can
    reach it, the browser still opens itself with the link, and isoshelf
    never asks you to invent a password.

### Fixed
- **Nothing here yet is a bug you would have hit** - but for the record, the
  login page needed its own line in the page's content policy, or it would
  have arrived as unstyled HTML. A test now checks the policy names exactly
  the stylesheet that is served.

## [v0.4.4] - 2026-09-22

### Fixed
- **A folder isoshelf can't write to now says how to fix it, at startup.**
  Before, it came out later as a wall of "permission denied" from whichever
  part of isoshelf happened to write first, saying what failed and nothing
  about what to do. isoshelf now checks both folders when it starts and says
  which folder is in the way, **which user it is running as** - the half
  nobody can work out from outside a container - what stops working until it
  is fixed, and the two ways to fix it on TrueNAS. It still starts either
  way: a folder it can only read is still listed and still checked for
  updates.
- **Reporting a problem gives you the details to paste.** The report opens
  GitHub's form with everything filled in, but GitHub's phone app ignores
  anything filled in from a link, so the form arrived empty. The dialog now
  shows the report as one block of text with a **Copy the details** button,
  and says to paste it if the form opens empty. Copying works on a NAS too,
  where the browser's own clipboard isn't available because the page isn't
  served over https.
- **Dialogs no longer put their buttons past the bottom of a phone screen.**
  The question box had no height limit, so on a short screen the answer
  buttons were somewhere below the edge with no way to scroll to them. It now
  caps and scrolls, the same as the folder chooser always has.

## [v0.4.3] - 2026-09-22

### Added
- **The folders isoshelf remembers, in the folder chooser.** Each one now says
  when you last looked at it, how many images it held and how much room they
  took, and the end of its path rather than the start - `…/tank/isos` and
  `…/template/iso` tell each other apart in a way `/mnt/tank/…` twice over
  does not. **Forget** takes one off the list; it throws away isoshelf's copy
  of what it found there and nothing else, so opening the folder again reads
  everything back from the folder itself.
  - None of it goes near a drive. A folder on that list may be a NAS that is
    asleep or a stick in a drawer, and opening the chooser must not go looking.

## [v0.4.2] - 2026-09-22

### Added
- **Choose where a folder's records live.** What isoshelf has worked out about
  a folder - its history, the images you starred, what each file turned out to
  be - normally lives in a `.isoshelf` folder inside it, so the drive carries
  that to whatever computer you plug it into next. In Settings, under *Where
  things are*, you can now keep it in isoshelf's own folder or one you pick
  instead, for a drive isoshelf shouldn't be writing to. Each folder answers
  for itself.
  - Changing the answer **moves** what isoshelf knows; the folder doesn't come
    back forgotten. Your images are never moved, and neither is the archive.
  - The one thing to know: kept away from the folder, the records are found by
    the folder's path, so the same drive at a different letter or mount point
    starts with nothing.

## [v0.4.1] - 2026-09-22

### Fixed
- **No more sideways scrolling on a phone.** The catalog's columns had a
  280-pixel minimum, and a card in a grid won't shrink below its own contents
  unless it is told it may - so on a narrow screen the widest thing on the
  page set the width of everything, and the whole page scrolled left and
  right. Every grid minimum can now give way to a narrower screen, and the
  top bar wraps rather than pushing *Settings* off the edge. Checked from 280
  pixels up, with the catalog open, the details panel open, Settings open and
  the filter menu open.
- **Opening a server's address without the link now says where the link is.**
  It said "go back to the isoshelf window and open that link" - which is right
  on your own computer and useless in a container, where there is no window
  and the link is in the app's log. It now says exactly that, and where to
  look on TrueNAS and with Docker. Found the first time someone typed the
  address in.

## [v0.4.0] - 2026-09-21

isoshelf can live on the machine your images already live on.

### Added
- **Run isoshelf on a NAS or home server, in a container.** A `Dockerfile`, a
  `docker-compose.yml` and [docs/docker.md](docs/docker.md), which walks
  through adding it to TrueNAS SCALE as a custom app - including the
  permissions snag that catches people out. Images are published for amd64
  and arm64. If your images live on a NAS, this is the way to run it: the
  work happens next to the files instead of across the network.
- **`--listen ADDRESS`**, which is what makes that possible. isoshelf normally
  answers only to localhost, because the page is for the person at the
  keyboard. Given an address, it answers to the machine's own name as well -
  and then the token in the link is the only thing keeping anyone out, so it
  says so plainly when it starts. Everything else is unchanged: the token, the
  cookie, the header on every change, and a same-origin check that now also
  accepts `https`, since a reverse proxy in front of a NAS speaks https to the
  browser and plain http to isoshelf.
- **The secret in the link makes itself, once.** On a server isoshelf writes
  it into its config folder the first time it starts and uses that one
  afterwards, so the link you bookmarked still works after a restart or an
  update - and installing it asks you to invent nothing. A box marked "token"
  on an install form gets `password` typed into it, and that box is the only
  thing between a stranger on your network and your images. Set
  `ISOSHELF_TOKEN` if you would rather choose it; it must be at least sixteen
  characters. On a desktop nothing changes: a fresh link every run, opened
  for you.
- **`/healthz`**, the one path that needs no token, for a container's health
  check. It answers `ok` and says nothing else: not which folder is open, not
  what is in it.

### Fixed
- **isoshelf says when the folder it was given isn't there.** It started
  silently with an empty folder chooser and no hint about why, which in a
  container - where the folder is named at startup and a mistyped mount is
  the commonest first mistake - left nothing at all to go on. It now names
  the folder and what is wrong with it, tells apart "nothing is there" from
  "that is a file", and adds a line about checking the mount when it is
  running as a server. It still starts, so you can choose a folder instead.
- **A folder isoshelf can't read no longer stops it starting.** If something
  other than "it isn't there" came back when it looked for your own catalog -
  a config folder belonging to another user, which is the usual way this goes
  wrong on a NAS - isoshelf decided you must have one and then failed trying
  to open it. It now says which path it couldn't read and why, and carries on
  with the built-in list of images. A permissions mistake looked like a crash
  before; now it looks like a message telling you which folder to fix.

## [v0.3.7] - 2026-09-21

### Changed
- **When you keep both copies, it's the new download that carries the
  version.** v0.3.6 did it the other way round: the file already on your
  drive was renamed and the new one took the unchanging name. This is the
  safer way round, and the maintainer's call - nothing already on the drive
  is touched at all, so nothing pointing at a file by name can break, and the
  new file says on its face which version it is. `netboot.xyz.iso` stays
  exactly where it was, and the new one arrives as
  `netboot.xyz-2.0.87.iso` (or with the date, when the project doesn't
  publish a version).

## [v0.3.6] - 2026-09-22

### Added
- **Keep both copies of an image whose filename never changes.** Updating
  netboot.xyz and the other fixed-name images used to mean choosing between
  the old file and the new one - *Keep both* was refused outright, because
  the new download wanted a name the old file already had. It no longer is:
  the old copy steps aside under a name of its own
  (`netboot.xyz-2026-09-01.iso`, or its version when isoshelf knows it) and
  the new download takes the unchanging name. That way round on purpose -
  anything pointing at the name that never changes, like a Proxmox VM, keeps
  working and quietly gets the newer image, and on a Ventoy drive both simply
  appear in the boot menu. The renamed file keeps its record, so it is still
  recognized rather than becoming an unknown file at the next scan.
- **Report a problem.** When something goes wrong there is now a button next
  to it that opens GitHub's bug form with the details already filled in:
  which isoshelf, which system, what kind of folder, and what isoshelf said.
  Nothing is sent anywhere by isoshelf - the report is read, edited and
  submitted by you, and closing the tab sends nothing. The dialog shows every
  line before the tab opens and lets the system details be left out. Your
  folder's location and the names of your files are never included.

### Fixed
- **The Filter menu no longer runs off the left of the screen on a phone.**
  It was held inside the window on the right but not on the left, so on a
  390-pixel screen the first filters were off the edge.
- **Escape closes an open menu.** It closed Settings, the details panel and
  the downloads bar, but left a menu hanging over whatever it closed.

### Changed
- **Plainer, shorter wording** through the page and its messages. *Carry on*
  is now *Resume*, which is what it does. A stopped download says "Stopped.
  What downloaded so far is kept, so Resume picks up from there" instead of
  explaining itself twice. Messages that go wrong still say when nothing in
  the folder has changed, because that is the part worth knowing.

## [v0.3.5] - 2026-09-21

### Added
- **Add a file from your own computer.** Drag an image anywhere onto the page,
  or choose one, and it goes straight into the folder - for the images no
  catalog will ever know: a Windows ISO you downloaded by hand, a recovery
  image your work gave you, something you built yourself. isoshelf works out
  what it is afterwards, the same way it does for a file copied in with
  Explorer. Only image files, only into the folder you chose, and a file of
  the same name is never written over without asking: the answers are the
  same two an update offers, *Archive the old one* or *Replace it*. The file
  arrives in `.isoshelf/incoming` and is only moved into place once all of it
  is there, so an upload that fails or is stopped leaves the folder exactly
  as it was.

### Fixed
- **A file replaced by one of the same name no longer vanishes from the
  archive.** It was moved aside correctly and kept on the drive, but the next
  scan dropped the note about it - the folder had that name in it again - so
  it disappeared from the page while still using room, with no way to restore
  it or see why the drive was fuller than the list suggested. The archive now
  lists what is actually waiting in `.isoshelf/removed`, whatever the notes
  say. This could happen to any image whose filename never changes, like
  `netboot.xyz.iso`, since v0.2.8.
- **The archive and the history are told apart by whether the file is still
  there**, not by whether it can be put back this minute. One waiting under a
  name a newer file has taken is in the archive, where it can be seen and
  emptied, and says what to do to get it back.

## [v0.3.4] - 2026-09-21

### Fixed
- **The page no longer locks up while images download.** A scan or *Refresh*
  was refused for as long as a queue took - which on a drive's worth of
  images is most of an evening - because a scan and a download shared one
  slot. They have a slot each now, and the page shows both at once: the scan
  card at the top, the downloads bar at the bottom. Switching folders and
  emptying the archive still wait for the downloads, because both pull the
  ground out from under one.
- **A scan can no longer undo what a download just wrote.** A scan reads the
  folder's records when it starts and saves them again at the end, which
  meant a file that landed in between lost its record - what it is, where it
  came from, when it arrived - and could be listed as an image that had left.
  A scan now saves the way every other writer already did: only what it
  learned goes onto the records as they are at that moment. This was reachable
  before this release too, when a scan followed a download closely enough.

## [v0.3.3] - 2026-09-21

Fixes and tidying from a run through the whole page on a full drive.

### Fixed
- **The Filter menu no longer hangs off the screen.** On a folder with a lot
  of kinds and architectures it ran past the bottom of the window with the
  last filters out of reach, because it was the one menu never told to keep
  itself inside the window. It now opens upwards when there's more room
  there, and scrolls when there isn't room either way.
- **Tick boxes line up.** In that menu each one sat at a different place,
  with its words pushed to the right, because the boxes were being stretched
  to fill the row.
- **A download that would land on top of a file you have now asks.** It used
  to fail with advice and a *Try again* button that failed the same way. It
  now offers the two answers — *Archive the old one* or *Replace it* — and
  says plainly that nothing in the folder has changed meanwhile.

### Changed
- **American spelling** throughout: *Favorites*, *color*.
- ***Restore*** is the word for bringing a file back from the archive, in the
  button, the page and the docs. It was "Put back".
- **Archive and History cards** are tidier: the filename on its own line, the
  buttons together at the end of the row, and *Page* is now *Download page*.
- **No *Download again* on a file that's still in the archive.** Restoring it
  is instant, costs nothing and gives back the very file that was there.
- **The downloads bar says how much is left**, not just how many: "12 waiting
  · 31.4 GB to download".
- **The folder line says what the images take up**: "42.5 GB in images · 68
  GB free of 252 GB".
- **Every file in a release carries its version** —
  `isoshelf-v0.3.3-windows-amd64.exe` — so two downloads can be told apart.
  The copies inside the portable zip keep the plain name, because that is
  what you run from the drive and what a future self-update replaces in
  place. Releases up to v0.3.0 attached plain names, so a link straight to
  `releases/latest/download/isoshelf-windows-amd64.exe` stops working; there
  is no such link in this project, and the older releases keep their files
  exactly as they are.

## [v0.3.2] - 2026-09-21

isoshelf now checks for updates by itself, and remembers the answers, so the
page is right the moment it opens instead of after you press a button.

### Added
- **Checking happens on its own**: when you open the page and after every
  scan. What each project says is written down and used for a day, so only
  the images nobody has asked about lately cost anything. Opening the page a
  second time asks nobody at all.
- **One Refresh button** in place of *Scan* and *Check for updates*. It reads
  the folder again and asks every project again, however recently it
  answered.
- **Settings → Checking for updates**: *Check for updates by itself*, which
  turns all of that off and leaves isoshelf offline until you press Refresh,
  and *Tell me when a new isoshelf is out*, which was only a command-line
  flag before. The first says underneath when the projects were last asked.

### Changed
- The line under the folder says how fresh the answers really are. A check
  that reused this morning's answers says this morning, not "just now".
- `isoshelf check` on the command line still asks every project, since typing
  it is asking, but what it learns is written down for the page.
- A project that couldn't be reached is never remembered, so a site that was
  down for a minute isn't bad news for the rest of the day.

## [v0.3.1] - 2026-09-21

Settings. Everything isoshelf lets you change is in one panel now, with a
search box, and nothing in it has to be touched for isoshelf to work.

### Added
- **A Settings panel**, from the button in the top bar or by pressing Escape
  to close it again. Each setting says what it is and what it does, and the
  search box finds one by name or by what it's for ("dark", "text", "bug").
- **How it looks:** *Light*, *Dark* or *Match this computer*; *Higher
  contrast* for a bright room or tired eyes; *Larger text*, a size up for the
  whole page without the browser's zoom; and *Less movement*, which stops the
  spinners and bars while isoshelf works.
- **What happens to the file an update replaces** — replace it, move it to
  the archive, or keep both — for every image that hasn't been given its own
  answer in its panel. This setting existed in the file but nothing read it;
  now it's the one the page starts from.
- **Where things are:** the folder isoshelf is watching, with the button to
  change it, and isoshelf's own folder, so nobody has to hunt for where the
  settings live.
- **Help:** the version you're running, whether a newer one is out, and
  *Report a bug*, which opens a new issue with the version already filled in.
  Nothing is sent until you press send yourself.
- **Reset to defaults**, which puts every switch back and keeps your folder,
  your pinned folders and everything isoshelf has learned about your images.

### Changed
- The colours are written once instead of twice, so choosing a theme is the
  page following your choice rather than your computer's.
- Sizes on the page are measured against one number, which is what lets
  *Larger text* move all of them together.
- The settings file has one owner (`internal/settings`) rather than a second
  copy of the fields in the web server. A choice made on the page could
  quietly erase one made on the command line; it can't now.
- The page is now one script per part of it — the list, the details panel,
  downloads, updating and identifying, the folder chooser, the archive, and
  Settings — instead of one 2,544-line file. Nothing about the page changed;
  it is a change for whoever works on it next.

### Removed
- Code nothing called any more: two helpers left over from features the
  redesign replaced, the sort orders for columns the page no longer has, a
  field sent to the page twice a second during a scan and read by nothing,
  and nine stylesheet rules for parts of the old table. Two dead-code
  checkers come back clean.

## [v0.3.0] - 2026-09-21

A new look. The page says what wants doing, in plain words, and everything
about an image is one click away instead of spread across ten columns.

### Added
- **Things to do, as cards at the top.** Updates ready, older versions you
  could clear, the archive, files that won't boot, unknown files, images
  missing, and "the folder changed". Each card says how much space is
  involved and has the button for it. The old row of banners is gone.
- **A checklist instead of a question.** "Update all" now shows every image
  it means, what happens to each one's old file, and how much it will
  download. Untick anything you'd rather leave. Clearing older versions uses
  the same checklist, with the newer file named beside each one.
- **A details panel.** Click any image: where it came from, its file, size
  and dates, the version here and the newest published, its links, what
  happens to old copies, and the buttons for it. The "…" menu is gone, and
  with it the way it used to get cut off at the bottom of the list.
- **Plain status words** — *Update ready*, *Old release*, *Check by hand*,
  *Won't boot here*, *Unknown file* — each explaining itself when you hover
  or tap, with a full key behind "What do the statuses mean?".
- **One Filter menu** with tick boxes for updates, favourites, older
  versions and ⚠ marks, plus the kinds and architectures this folder
  actually holds. Whatever is on shows as a chip you can remove, so a short
  list always says why it's short.
- **A jump bar** across the top: Your images, Add images, Archive, History,
  with counts.
- **Each image decides what happens to its old file**, once: replace,
  archive, or keep both, in its details panel. Updates stop asking.
- **Say which version a file is** for images whose name doesn't say and
  whose project publishes nothing to compare against, like Hiren's BootCD.
- **A layout for phones.** The list becomes a card per image, with no
  sideways scrolling.

### Changed
- **Archive and History are separate.** *Archive* holds files isoshelf set
  aside: still on your drive, still using room, with Put back and Empty.
  *History* is a record of images that left, with Download again. The old
  "Images that were here" mixed the two.
- The date column now says **Updated** for images isoshelf replaced, and
  **Added** for files that simply turned up.
- The list is four columns wide: the image (with its file, size and date
  underneath), its version, its status, and what you can do.

### Fixed
- A new isoshelf release could take up to a day to be noticed, because the
  answer from GitHub was remembered for 24 hours. It is now remembered for an
  hour, so opening isoshelf shortly after a release shows it.
- Removing or archiving a file no longer makes isoshelf announce that the
  folder changed; it knows it was the one who changed it.

## [v0.2.9] - 2026-09-18

### Changed
- **Downloads no longer hold everything else up.** While images download you
  can remove, archive, identify ("What is this?" and "Not right?"), put back
  and clear older copies, as well as star images and flip replace switches.
  Only scanning, switching folders and emptying the archive wait for the
  downloads. Whatever you change is saved beside the downloads' own changes,
  never over them.

### Fixed
- The "…" links menu no longer gets cut off near the bottom of the list: it
  opens upwards when there's no room below, and closes when you scroll.
- "Show them" on older copies becomes **Show all images** while those are
  showing, so the list never gets stuck with just them.
- Archiving a file says where it went ("Images that were here", at the bottom
  of the page) and opens that section.
- An update no longer fails when you removed its old file by hand while the
  new one was downloading.
- Something you scroll or tab to no longer ends up hidden behind the
  Downloads bar.

## [v0.2.8] - 2026-09-18

### Added
- **A download queue**, like a game launcher's. Add and Update no longer wait
  for the download that's running: they join a queue, and images download one
  at a time. A **Downloads** bar along the bottom of the page shows the one
  running, with its speed and time left. Open it to change the order (drag,
  or use the arrows), take one off the queue, or stop them all.
- **Buttons say what's happening.** An Add or Update button turns into
  *Queued #2*, *Downloading 45%* or *Added ✓*, and after a failure offers
  *Try again*. A stopped download offers *Carry on*, and picks up where it
  stopped.
- **Update all** puts every update on the queue at once, and asks once what
  to do with the old files, in the same words as a single update.
- "Fits in this folder" counts the downloads already waiting.
- Stars and the replace switch keep working while downloads run.

### Changed
- The folder is scanned once when the queue is empty, rather than after every
  download.
- While something runs, the page only redraws what moves, so an open menu
  stays open and the keyboard keeps its place.

### Fixed
- **A download that couldn't be checked could replace your old file.** The
  rule was always that it mustn't, and now the code enforces it: when a
  project publishes no checksum, the old file stays where it is (or, if the
  new one has the same name, it's archived, never deleted), and the
  Downloads list says why. No image in the built-in catalog is affected; it
  matters for catalogs of your own.
- Updating an image whose filename never changes (like `netboot.xyz.iso`)
  failed when its replace switch was off. The answer you give is followed
  now.
- A download that a server interrupted with an HTTP/2 stream reset gave up at
  once. It now retries, and carries on where it stopped.

The list of images changed too: Kali Linux live is now a link to Kali's page,
because Kali offers it only as a torrent. See
[CATALOG-CHANGES.md](CATALOG-CHANGES.md).

## [v0.2.7] - 2026-09-18

### Added
- **The catalog has its own changelog**, [CATALOG-CHANGES.md](CATALOG-CHANGES.md):
  dated, no version numbers, saying which images were added and what changed.
  The list of images updates itself separately from isoshelf, so this is the
  one place those changes are written down. The page says when the list last
  changed, with a *What's new* link.
- A **Popular** filter in the catalog, beside "Fits in this folder".

### Fixed
- Long image names no longer leave their badges dangling at the end of a
  wrapped line: the architecture and "popular" badges sit on a line of their
  own, underneath.
- Filenames wrap at their `_`, `-` and `.` instead of in the middle of a word.

## [v0.2.6] - 2026-09-18

### Added
- **A ⚠ mark for images worth knowing about**, with a one-line key under the
  list — like the symbols on a menu. It marks releases that no longer get
  security fixes, and a few images with a note: CentOS 7 (end of life since
  June 2024), Windows 10 (out of support since October 2025), AtlasOS (an
  unofficial modification of Windows) and Windows Insider Preview builds
  (which expire). Hover the mark for the reason. It never blocks anything:
  keeping an old image for a VM or an old PC is a perfectly good reason to
  keep it.
- **An Added column**: when each file arrived in the folder, sortable, so
  three copies of Windows can be told apart by age. On Windows and network
  shares this is the file's own creation date; elsewhere, isoshelf notes
  when it first saw a new file.

### Changed
- "Recently changed" in the sort menu is now **Recently added**. A copied
  file keeps its old change date, so it never answered that question.

## [v0.2.5] - 2026-09-18

### Added
- **Two new kinds:** *Gaming and handhelds*, and *Raspberry Pi and other
  boards* — which is where the Pi filter lives. "Desktop Linux" is now just
  *Desktop*.
- **Seven more images**, 86 in all: Omarchy; Nobara in its Official (KDE),
  GNOME and Steam Handheld editions; NixOS; EndeavourOS; and Zorin OS Core.
  Omarchy and Nobara can be downloaded and checked. EndeavourOS and Zorin
  publish their checksums only on mirrors, so for now they're a link to the
  project's download page; NixOS gets update and end-of-life checks with a
  link.

### Changed
- Images that can only be linked to are now welcome in the catalog: isoshelf
  still recognizes the file and sends you to the right page.
- isoshelf can download images from servers that don't list their files, by
  using the filename the project's own download page gives.

## [v0.2.4] - 2026-09-18

### Added
- **Older copies are easy to clear.** When a folder holds more than one
  version of the same image — one downloaded by hand next to one isoshelf
  fetched, say — a line above the list says how many and how much room they
  use, with **Show them** and **Clear them**. Clearing asks once, lists every
  file it means, and offers archive or delete as usual.
- An **Older copies** filter beside Updates and Favorites.
- **isoshelf notices when the folder changes.** Drop a file in with Explorer
  while isoshelf is open, and a line offers to scan again.

### Fixed
- A bad edit could leave the page script unable to start. Tests now check the
  script's shape, that every element it looks for exists, and that every link
  opening a new tab does so safely.

## [v0.2.3] - 2026-09-18

### Added
- **Sort by any column** — status, image, version, latest, file, size, and
  the replace switch. Click again to reverse. Empty cells always sort last.
- **Size has its own column**, so "what's using all the room" is one click.
- **Update all** has its own line above the list, saying how many images
  have updates.

### Changed
- When an update replaces a file, the choice reads **Replace it (frees
  2.3 GB)** or **Archive it (still uses 2.3 GB, undo any time)**, and isoshelf
  remembers which you usually pick. When the drive is too full for both,
  Replace comes first and says why.
- The catalog's "can be downloaded" tick is now a filter by how an image
  updates: downloads and checks, update checks only, or download page only.

### Fixed
- Catalog cards were squashed into four narrow columns. Three wider ones now.

## [v0.2.2] - 2026-09-18

### Added
- The folder chooser shows each drive's name ("Z: — red14") and how many
  images a place holds.
- **Bookmarks** for folders you use often.

### Changed
- **Your images appear straight away.** Checking fixed-name images (like
  `netboot.xyz.iso`) carries on behind the list instead of holding it up. It
  happens once per file and is remembered.

## [v0.2.1] - 2026-09-18

### Added
- A third kind of folder, **Folder of images**, for a NAS share, a downloads
  folder or a stash of card images. Nothing there is called "not bootable",
  and compressed images (`.img.xz`) count. It's the default now; a real Ventoy
  drive is recognized by its `ventoy` folder.
- **Refresh** in the folder chooser, for a drive plugged in after isoshelf
  started.
- **Report a problem** on every image, for downloads that move or break, and
  a link at the bottom of the catalog to ask for an image it doesn't know.
- Seven more images: Rocky Linux, AlmaLinux, Proxmox Mail Gateway, Raspberry
  Pi OS (desktop and Lite), and Home Assistant OS for the Pi 5 and x86.
- A **popular** marker and "Popular first" sorting in the catalog: a
  hand-picked hint from public round-ups, not a rating.

### Fixed
- Catalog revision numbers now always sort in the right order.

## [v0.2.0] - 2026-09-18

The first release.

- Scans a Ventoy drive, a NAS share or Proxmox ISO storage and works out which
  distribution, edition, architecture and version each image is.
- Checks each one for updates, using endoflife.date, GitHub releases, and the
  projects' own download listings.
- Downloads updates, checks them against the checksums the projects publish,
  and only then puts them in place. Interrupted downloads carry on.
- Removes images you don't want, with an archive you can put them back from.
- Works out what unrecognized files are, and lets you name the ones no list
  will ever know.
- Adds images from a catalog of 72, showing their size and the room you have.
- Keeps that catalog current from this repository without a new release.
- Runs as a page in your browser, or from the command line.

[v0.3.0]: https://github.com/ZachCurry13/isoshelf/releases/tag/v0.3.0
[v0.2.9]: https://github.com/ZachCurry13/isoshelf/releases/tag/v0.2.9
[v0.2.8]: https://github.com/ZachCurry13/isoshelf/releases/tag/v0.2.8
[v0.2.7]: https://github.com/ZachCurry13/isoshelf/releases/tag/v0.2.7
[v0.2.6]: https://github.com/ZachCurry13/isoshelf/releases/tag/v0.2.6
[v0.2.5]: https://github.com/ZachCurry13/isoshelf/releases/tag/v0.2.5
[v0.2.4]: https://github.com/ZachCurry13/isoshelf/releases/tag/v0.2.4
[v0.2.3]: https://github.com/ZachCurry13/isoshelf/releases/tag/v0.2.3
[v0.2.2]: https://github.com/ZachCurry13/isoshelf/releases/tag/v0.2.2
[v0.2.1]: https://github.com/ZachCurry13/isoshelf/releases/tag/v0.2.1
[v0.2.0]: https://github.com/ZachCurry13/isoshelf/releases/tag/v0.2.0

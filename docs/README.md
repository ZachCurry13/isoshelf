# The documents

What each file here is for, so you can go straight to the one you want.

| File | What it holds |
|---|---|
| [design.md](design.md) | The whole design and every rule isoshelf follows: what it will and won't touch on your drive, how an update is verified before it replaces anything, how the catalog works, and why each of those is the way it is. The long one, and the one to read if you want to understand the program. |
| [catalog-sources.md](catalog-sources.md) | Where each image in the catalog comes from: which page or API says what the newest version is, where its checksum is published, and the awkward cases. Read this before adding an image. |
| [STATUS.md](STATUS.md) | Where things stand and what has been decided, newest first. The record of choices and why they were made. |
| [TODO.md](TODO.md) | What's next, in order, with what you need to know before starting each one. |
| [docker.md](docker.md) | Running isoshelf on a NAS or home server, and adding it to TrueNAS SCALE as a custom app. |
| [design-audit.md](design-audit.md) | A critical look at the page as a piece of design, written before the v0.4 redesign. A proposal with open questions, not a plan that was agreed. |

The changelogs live in the root: [CHANGELOG.md](../CHANGELOG.md) for isoshelf
itself, and [CATALOG-CHANGES.md](../CATALOG-CHANGES.md) for the list of
images, which updates itself separately from releases.

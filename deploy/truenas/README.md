# The TrueNAS app, in progress

The goal is that by 1.0 isoshelf installs from TrueNAS's own Apps screen
rather than as a custom app. That means an entry in
[truenas/apps](https://github.com/truenas/apps), under
`ix-dev/community/isoshelf/`. These files are that entry, kept here so they
are versioned alongside the thing they describe.

**Nothing here has been submitted, and none of it has been run.** Read the
next section before doing either.

## What is real and what isn't

**Taken from a real community app in `truenas/apps`, so the shape is right:**

- `app.yaml` — the manifest: name, version, train, who maintains it, and the
  user it runs as. TrueNAS runs apps as uid/gid 568 (`apps`), which is why
  that number appears here and in `docs/docker.md`.
- `item.yaml` — categories, tags and the icon the store shows.
- `ix_values.yaml` — the image to pull and the constants the templates use.

**Not written, deliberately:**

- `questions.yaml` — the install form. Every option a person sees when they
  install it, in TrueNAS's own schema.
- `templates/docker-compose.yaml` — what those answers turn into. TrueNAS
  apps don't write plain compose: they call a shared template library, and
  which helpers exist depends on the `lib_version` pinned in `app.yaml`.

Both of those have to be written against the library version current in
`truenas/apps` at the time, by someone who can read one of their templates
in full and run `docker compose config` on the result. Guessing at them
would produce something that looks right and isn't, and a broken app in a
store is worse than no app in a store.

**Also still missing: an icon.** `item.yaml` points at `icon.png` in this
folder, which does not exist yet. It should be a real mark, not a letter in
a box.

## What the install form has to do

Whoever writes `questions.yaml` should keep these in mind, because they are
the difference between an app people trust and one that quietly puts
somebody's files on the network:

- **Two storage questions**, both required: the folder of images, and a place
  for isoshelf's own files. Neither has a sensible default, and the second
  one is the one people forget — without it, settings and the secret for the
  link are lost every time the app updates.
- **Do not ask for a token.** isoshelf makes one on first start and keeps it
  in its config folder. A box on an install form marked "token" gets
  `password` typed into it, and that box is the only thing standing between
  a stranger on the network and somebody's images. `ISOSHELF_TOKEN` exists
  for people who want to choose, and can be an advanced option at most.
- **Say where the link is.** The first thing someone needs after installing
  is the link with the secret on it, and it is printed in the app's log. The
  app's description should say so outright, because nobody thinks to look
  there.
- **Say that the folder is the container's, not the host's.** In the folder
  chooser, the dataset mounted at `/images` appears as `/images`. That is
  the one thing about running in a container that surprises people.

## Before submitting anything

1. Build and run the image — see the note in `docs/TODO.md`; it has never
   been built, because the machine these files were written on has a Docker
   client and no daemon.
2. Install it on a real TrueNAS box as a custom app first, following
   `docs/docker.md`, and check the permissions advice is actually right.
3. Then write the two missing files, render them, and install from a local
   catalog before opening a pull request against somebody else's repository.

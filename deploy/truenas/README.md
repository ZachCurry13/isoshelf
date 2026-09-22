# The TrueNAS app, in progress

The goal is that by 1.0 isoshelf installs from TrueNAS's own Apps screen
rather than as a custom app. That means an entry in
[truenas/apps](https://github.com/truenas/apps), under
`ix-dev/community/isoshelf/`. These files are that entry, kept here so they
are versioned alongside the thing they describe.

**Nothing here has been submitted.** The image itself is real - it is
published, and isoshelf has been installed on a TrueNAS box as a custom app
- but these catalog files have never been rendered or installed from. Read
the next section before treating them as finished.

## What the catalog actually requires

Read from `CONTRIBUTIONS.md` in `truenas/apps` on 2026-09-22, rather than
assumed. Re-read it before submitting: it is their repository and it moves.

- **New contributions go in the `community` train**, and only there. The
  others are iXsystems'. `app.yaml` already says `train: community`.
- **Six files are required**, and this folder has three:
  | File | Here? |
  |---|---|
  | `app.yaml` | yes |
  | `ix_values.yaml` | yes |
  | `README.md` | yes |
  | `questions.yaml` | **no** |
  | `templates/docker-compose.yaml` | **no** |
  | `templates/test_values/basic-values.yaml` | **no** |
- **Image tags must be pinned**, never `latest`. `ix_values.yaml` pins the
  version, and `internal/docs` fails the build if it falls behind the
  changelog - so that stays true without anybody remembering.
- **ghcr is preferred over docker.io.** isoshelf publishes to ghcr already.
- **Nothing in their rules requires the app to have reached 1.0.** Being
  before 1.0 is not a reason the catalog would refuse isoshelf. Whether it
  is a good idea is a different question, and the answer is below.

## Should a pre-1.0 isoshelf go in the catalog?

Their rules allow it. The argument against doing it yet is not about rules:

- **The catalog is where strangers find it.** They will not read this
  repository, the changelog, or the pre-release badge on GitHub. They will
  install it from a list and expect it to behave.
- **Being before 1.0 means things can still change under people.** That is
  exactly what the pre-release label warns about, and catalog users never see
  that label.
- **Once it is listed, the image has to keep working.** A tag that moves or
  breaks breaks it for everyone who installed it, and that doesn't end when
  the pull request is merged.

The custom-app route in `docs/docker.md` already works and asks nothing of
anybody else. The sensible order is: finish what 1.0 means, then submit.

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
- `templates/test_values/basic-values.yaml` — the values their tests render
  the template with.

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
- **Offer the username and password, and say they are optional.** Since
  v0.4.5 isoshelf asks for them the first time the page is opened, so the
  form needs nothing. `ISOSHELF_USERNAME` and `ISOSHELF_PASSWORD` set them
  ahead of time and close the window where whoever arrives first chooses, so
  they belong on the form - as two ordinary fields, with the password one
  marked `private: true`.
- **Do not ask for a token.** isoshelf makes one on first start and keeps it
  in its config folder. A box on an install form marked "token" gets
  `password` typed into it. `ISOSHELF_TOKEN` exists for people who want to
  choose, and is an advanced option at most.
- **Say that the first thing to do is open it.** The description should say
  that opening the address asks for a username and password, and that the
  app's log also holds a link that gets in without one - because nobody
  thinks to look there, and that link is the way back from a forgotten
  password.
- **Say that the folder is the container's, not the host's.** In the folder
  chooser, the dataset mounted at `/images` appears as `/images`. That is
  the one thing about running in a container that surprises people.

## Before submitting anything

1. ~~Build and run the image.~~ *Done:* the `container image` workflow builds
   it for amd64 and arm64 and pushes it to `ghcr.io/zachcurry13/isoshelf`.
2. ~~Install it on a real TrueNAS box as a custom app~~, following
   `docs/docker.md`. *Done:* the walkthrough is what the maintainer followed,
   and the two things it got wrong have been fixed since.
3. Write the two missing files, render them, and install from a local catalog
   before opening a pull request against somebody else's repository. An icon
   too — see above.

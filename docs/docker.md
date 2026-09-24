# Running isoshelf on a NAS or home server

isoshelf normally runs on the computer in front of you. In a container it runs
somewhere else — a TrueNAS box, an unRAID box, a Pi in a cupboard — and you
open its page from your own machine. That is the only real difference, and
this page is about the handful of things that follow from it.

If your images live on a NAS, this is probably the way you want to run it:
isoshelf reads and writes the files where they already are, over the local
disk rather than over the network, which for a 6 GB image is the difference
between minutes and hours.

## What it needs

Two folders from the host, both mounted into the container:

| Inside the container | What it is |
|---|---|
| `/images` | The folder of bootable images to look after. |
| `/config` | isoshelf's own files: settings, its copy of the catalog, and the record of what each folder held. Losing it doesn't lose any images — but it does lose the history and your settings, so mount it somewhere real rather than leaving it in the container. |

You don't have to set anything else.

## Getting in: a username and password

**Open the address and isoshelf asks you to choose a username and password.**
That is the first run: pick them, and from then on every device on your
network signs in with them. A sign-in lasts a month and survives restarts, so
updating the app doesn't sign you out.

Two things worth knowing:

- **Until somebody sets them, whoever opens the address first is the one who
  chooses.** isoshelf says so in its log, with the address, so it isn't a
  surprise. Do it as soon as the app is up rather than later.
- **To skip that window entirely**, set `ISOSHELF_USERNAME` and
  `ISOSHELF_PASSWORD` before it ever starts — in the app's environment
  variables, where you are already filling in a form. isoshelf then has a
  login from its very first second and never offers to set one.

Settings → *Sign-in* changes the username or password later, signs
this browser out, or signs every browser out at once.

### There is no token once you have a password

Before anyone has set one, isoshelf prints a link with a secret on the end in
the container's log, so a brand-new install can be reached at all. **That link
stops working the moment a password is set** — a login replaces it rather than
sitting beside it, because two ways in is two ways in, and the weaker of the
two was sitting in a log.

### If you forget the password

Neither way back goes through the page, and both need the machine itself,
which is the point:

- **Set `ISOSHELF_USERNAME` and `ISOSHELF_PASSWORD`** in the app's environment
  variables and restart it. Those overwrite whatever was there.
- **Or run it from a shell on the machine:**

  ```sh
  docker exec -it isoshelf isoshelf password
  ```

  It asks for a new password, keeping the username you already have. What you
  type shows on the screen — use the environment variables if that matters.

## What this is and isn't

On your own computer, isoshelf answers only to `localhost`. In a container it
has to answer to the machine's address, or you could never reach it.

- **Put it on a network you trust.** This is a home-lab tool. It has one
  login, not user accounts, and it is not built to face the internet.
- **Anyone who gets in can add, replace and delete images in that folder.**
- **Over plain `http` a password travels unencrypted.** On your own LAN that
  is the same exposure as every other NAS app; over the internet it is not
  good enough. If you want it reachable from outside your house, put it
  behind a VPN, or a reverse proxy with `https`. isoshelf works behind a
  proxy on `https` without any extra configuration.

## Docker Compose

```yaml
services:
  isoshelf:
    image: ghcr.io/zachcurry13/isoshelf:latest
    container_name: isoshelf
    restart: unless-stopped
    ports:
      - "8765:8765"
    # No environment needed: isoshelf makes its own secret, keeps it in
    # /config and prints the link in the log. Set ISOSHELF_TOKEN if you would
    # rather choose it yourself.
    volumes:
      - /mnt/tank/isos:/images
      - /mnt/tank/appdata/isoshelf:/config
    # The image runs as 1000:1000 by default. isoshelf works as any user, so
    # set this to whoever owns the folders rather than opening them up. On
    # TrueNAS that is usually 568:568.
    user: "1000:1000"
```

Then `docker logs isoshelf` prints the link to open, secret and all.

## TrueNAS SCALE, as a custom app

Written for TrueNAS SCALE 24.10 (Electric Eel) or later, where apps are
Docker. The menus move between versions; the five things you are filling in
don't.

**Before you start**, make a dataset for isoshelf's own files — something like
`tank/appdata/isoshelf`. You should already have one holding your images.

1. **Apps → Discover Apps → Custom App.**
2. **Name** it `isoshelf`.
3. **Image**: repository `ghcr.io/zachcurry13/isoshelf`, tag `latest`. You can
   pin a version instead — `vX.Y.Z`, whichever is current — if you would rather decide when it
   changes.
4. **Environment variables**: none needed — the first time you open it,
   isoshelf asks you to choose a username and password. To have them set
   before it ever starts, add `ISOSHELF_USERNAME` and `ISOSHELF_PASSWORD`
   here.
5. **Networking**: publish container port `8765` on host port `8765`. Pick a
   different host port if something else is already using it.
6. **Storage**: two host-path mounts.
   - your images dataset → `/images`
   - your appdata dataset → `/config`
7. **User and Group ID**: `568` / `568`. That is TrueNAS's own `apps` user,
   and it is what owns app datasets there unless you changed it. (The image
   defaults to 1000 because that is the common one everywhere else; isoshelf
   runs happily as any user, so set this to whatever owns your datasets.)
8. Install it and wait for it to go green, then open
   `http://your-truenas:8765/` and choose a username and password. (If you
   set them in step 4, sign in with those instead.)

Until you set that password, the app's **Logs** hold a link with a secret on
the end, which is how a fresh install can be opened at all. It stops working
as soon as a password exists.

### If downloads fail with "permission denied"

A folder isoshelf can list but not write to looks completely fine until the
first download, which fails with something like:

```
open /images/.isoshelf/partial/whatever.iso.part: permission denied
```

`.isoshelf` is isoshelf's own folder inside your images folder: downloads are
staged there, the archive lives there, and what isoshelf has learned about
the folder is kept there. If it was made on an earlier run — by whichever
user the app ran as then — and you have since changed the app's **User and
Group ID**, that folder is still owned by the old user while everything
around it is fine.

isoshelf says so at startup now, in the **Logs**, and names the user that owns
it. The fix is to hand that one folder over, on the host:

```sh
chown -R 568:568 /mnt/tank/your-images-dataset/.isoshelf
```

**Don't change the app's user to match it instead** — the folder around it is
already right, and that would break the part that works. If the folder holds
nothing you want, deleting it works too: isoshelf makes a new one, owned by
the right user, at the next scan.

### If it starts but can't write

This is a permissions problem rather than an isoshelf one: the user the
container runs as doesn't own the datasets you mounted, so isoshelf can see
your images but can't write to the folder.

isoshelf checks both folders when it starts and says so in the **Logs**,
before anything has gone wrong. It names the folder that is in the way, the
user it is running as, and what stops working until it is fixed:

```
isoshelf: can't write to /config, which is where isoshelf keeps its own files: ...
isoshelf:   Until that is fixed: the link's secret changes every restart, settings aren't
isoshelf:   remembered, and the copy of each folder's history isn't kept.
isoshelf:   To fix it, /config has to belong to the user isoshelf runs as (user 568, group 568).
```

It carries on either way — a folder it can only read is still listed and
still checked for updates — so a permissions mistake never looks like a
crash.

To fix it, make the two match:

- **Find the dataset's owner** (Datasets → your dataset → Permissions).
- **Put that UID and GID into the app's user and group fields**, or change the
  dataset's ownership to the user isoshelf names in the log.

On TrueNAS both are usually `568` (`apps`). Change it on the TrueNAS side,
not inside the container: those folders are mounts, and their permissions
come from the dataset.

**The `/config` mount has to be writable too, not just `/images`.** That is
where isoshelf keeps its settings and the secret for your link — if it can't
write there, the link changes every time the app restarts, which is the one
symptom people notice first.

### If the images folder is read-only

Mount it read-only if you like — isoshelf will scan it and tell you what is
out of date, it just can't change anything. One thing needs moving first:
what it works out about the folder normally goes in a `.isoshelf` folder
inside it. In **Settings → Where things are → Where this folder's records
are kept**, choose *In isoshelf's own folder*, and it keeps them in `/config`
with everything else of its own.

### Checking it is alive

`http://your-truenas:8765/healthz` answers `ok` and needs no token. It says
nothing else — not which folder is open, not what is in it — so it is safe to
point a monitor at. The container's own health check uses it.

## Keeping the images up to date without doing anything

A container is the one place isoshelf can do this properly, because it is
always on, so the first time you open the page it asks whether it should:
every day, every week, or only when you press Update. Until you answer, it
doesn't. Change your mind any time in **Settings → Checking for updates →
Update images automatically**. Every day or every week it checks, downloads
what it finds, verifies it against the project's published checksum and puts
it in place. The old copy goes the way Settings says — replace, archive or
keep both — and a file you pinned is never touched, so nothing happens that
the page hasn't been saying it would. Saying yes starts the first run
straight away, and the question says beforehand how many updates that is.

It stops before filling the folder rather than using the last of it, and it
leaves a folder alone while you are scanning or downloading into it yourself.
It is off until you turn it on.

## Letting your other machines copy from it

The machine holding the images is the one worth copying from. Turn on
**Settings → Share this folder with other isoshelfs** here, and on your laptop
turn on **Copy from another isoshelf first**, giving it this machine's address
and the login for it.

After that, an image this one already has arrives over your own network in a
minute instead of over the internet in an hour. Anything it hasn't got is
downloaded as before. The laptop's **Add images** marks what this machine
has, and copies any of it with **Copy** — including images the catalog
doesn't know, in a folded **Also on your server** list at the bottom.

A copy of a file the project publishes no checksum for can only be checked
against this machine's own copy. It arrives saying exactly that, with what
this machine knew about where it came from, and like any unverified file it
never replaces anything.

**Every file is still checked against the project's own published checksum**,
wherever the bytes came from. That is what makes the local copy safe to
prefer rather than something to trust: a copy that is stale, damaged, or
served by something pretending to be an isoshelf fails the same check any
download would, and costs one fall back to the real source.

Two things worth knowing:

- **Sharing is off until you turn it on**, because it hands whole images to
  anyone who can sign in — a different thing from letting them manage the
  folder.
- **What you share is yours to mind.** isoshelf hands over whatever is in
  the folder, the way any file share would; the licenses of the images in it
  are between you and their publishers.
- **The first scan after turning it on takes longer.** isoshelf hashes every
  image rather than only the ones whose filename never changes, because a
  hash is how another isoshelf asks for one particular file. Images this
  isoshelf downloaded itself are already hashed.

This machine also keeps a note of which drives have copied from it and what
each one took — visible in Settings, and the beginning of being able to
rebuild a drive that is lost.

## Updating it, without reinstalling

Nothing in `/images` or `/config` is touched by an update: they are mounts
from your own datasets, so your images, settings, history and login all stay
exactly where they are. Updating replaces the program and nothing else.

**On TrueNAS**, the reliable way is to name the version you want:

1. **Apps → isoshelf → Edit.**
2. Change the image **tag** to the new version — `vX.Y.Z`, whichever is
   current on [the releases page](https://github.com/ZachCurry13/isoshelf/releases).
3. **Save.** It pulls the new image and restarts the app.

That always works, because the tag differs from the one already on disk.

If you used `latest` instead, Edit → Save may or may not fetch a newer image:
whether it re-checks the registry depends on the pull policy your version of
TrueNAS uses. Stopping and starting the app does **not** fetch one — Docker
serves the copy it already has. So if you follow `latest` and want to be
sure, change the tag to the version number, save, and change it back if you
prefer.

**With Docker Compose** it is two commands:

```sh
docker compose pull
docker compose up -d
```

Either way, isoshelf tells you when a new one is out: the top bar shows it,
and Settings → *Tell me about new isoshelf versions* turns that off.

On a desktop, isoshelf can install a new version itself with **Update now**.
In a container it doesn't offer that, and says so: the next pull would put
the old program back, so a container is always updated by pulling the new
image, as above.

## Building the image yourself

```sh
git clone https://github.com/ZachCurry13/isoshelf
cd isoshelf
docker build -t isoshelf --build-arg VERSION=$(git describe --tags --always) .
```

## Looking after more than one folder

The folder chooser works here the same as anywhere — but it shows the
container's paths, not your NAS's. The dataset you mounted as
`/mnt/tank/isos:/images` appears in the chooser as `/images`, and folders you
didn't mount don't appear at all, because as far as the container is
concerned they aren't there.

So to look after a second folder, mount it too:

```yaml
    volumes:
      - /mnt/tank/isos:/images
      - /mnt/tank/proxmox/template/iso:/images2
      - /mnt/tank/appdata/isoshelf:/config
```

Then **Choose folder…**, under **Settings → This folder**, offers both, and
isoshelf remembers which one you were last in. Bookmark them if you switch
often. (On a server the card above the list is one line — when it last
checked, and how full the drive is — so the folder, its type and choosing
another live in Settings.)

## What is different in a container

- **Portable mode** has no meaning here; it is for a USB stick carried
  between machines.
- **isoshelf can't open a browser for you.** There isn't one. It prints the
  link in the container's log instead.

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

Settings → *Who can get in* changes the username or password later, signs
this browser out, or signs every browser out at once.

### The link, which still works

isoshelf also prints a link with a secret on the end, in the container's log:

```sh
docker logs isoshelf
```

That link gets you in without the password, and it is **the way back in if
you forget it** — so keep the log reachable. It is no weaker than the
password: both are in reach of anyone who can read the container's log or its
config folder. isoshelf makes the secret itself on first start and keeps it
in `/config`, so it doesn't change when the app restarts. Set `ISOSHELF_TOKEN`
to choose it yourself; sixteen characters is the minimum.

Once you're in, Settings → *Who can get in* → *Change username or password*
doesn't ask for the old one if you arrived by the link — which is the whole
point of it.

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

The app's **Logs** also hold a link with a secret on the end. You don't need
it to get in, but it is the way back if you forget the password, so it is
worth knowing it is there.

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

Mount it read-only if you like - isoshelf will scan it and tell you what is
out of date, it just can't change anything. One thing needs moving first:
what it works out about the folder normally goes in a `.isoshelf` folder
inside it. In **Settings → Where things are → Where this folder's records
are kept**, choose *In isoshelf's own folder*, and it keeps them in `/config`
with everything else of its own.

### Checking it is alive

`http://your-truenas:8765/healthz` answers `ok` and needs no token. It says
nothing else — not which folder is open, not what is in it — so it is safe to
point a monitor at. The container's own health check uses it.

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
and Settings → *Tell me when a new isoshelf is out* turns that off.

## Building the image yourself

```sh
git clone https://github.com/ZachCurry13/isoshelf
cd isoshelf
docker build -t isoshelf --build-arg VERSION=$(git describe --tags --always) .
```

## Looking after more than one folder

The folder chooser works here the same as anywhere - but it shows the
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

Then **Choose folder…** offers both, and isoshelf remembers which one you were
last in. Bookmark them if you switch often.

## What is different in a container

- **Portable mode** has no meaning here; it is for a USB stick carried
  between machines.
- **isoshelf can't open a browser for you.** There isn't one. It prints the
  link in the container's log instead.

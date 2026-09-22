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

You don't have to set anything else. isoshelf makes the secret for its own
link the first time it starts and keeps it in `/config`, so the link you
bookmark still works after a restart or an update. Read it from the
container's log:

```sh
docker logs isoshelf
```

If you would rather choose the secret yourself — to put it in a password
manager, or to keep it the same across a rebuild — set `ISOSHELF_TOKEN` to
something long and random and isoshelf will use that instead. Sixteen
characters is the minimum it will accept.

## The thing to understand about the link

On your own computer, isoshelf answers only to `localhost`. In a container it
has to answer to the machine's address, or you could never reach it — so
**the token in the link is the only thing keeping other people out.**

That means:

- Treat the link like a password. Anyone on your network who has it can add,
  replace and delete images in that folder.
- Put it on a network you trust. This is a home-lab tool; it has no user
  accounts, and it is not built to face the internet.
- If you want it reachable from outside your house, put it behind something
  that does authentication properly — a VPN, or a reverse proxy with a login
  in front of it. isoshelf works behind a proxy on `https` without any extra
  configuration.

Generate a token with something like:

```sh
head -c 32 /dev/urandom | base64
```

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
   pin a version instead — `v0.4.0` — if you would rather decide when it
   changes.
4. **Environment variables**: none needed. isoshelf makes its own secret and
   keeps it in `/config`. (Set `ISOSHELF_TOKEN` if you would rather choose
   it.)
5. **Networking**: publish container port `8765` on host port `8765`. Pick a
   different host port if something else is already using it.
6. **Storage**: two host-path mounts.
   - your images dataset → `/images`
   - your appdata dataset → `/config`
7. **User and Group ID**: `568` / `568`. That is TrueNAS's own `apps` user,
   and it is what owns app datasets there unless you changed it. (The image
   defaults to 1000 because that is the common one everywhere else; isoshelf
   runs happily as any user, so set this to whatever owns your datasets.)
8. Install it and wait for it to go green. Then open its **Logs**: isoshelf
   prints the link with its secret on the end. Open that, and bookmark it.

### If it starts but can't write

This is a permissions problem rather than an isoshelf one: the user the
container runs as doesn't own the datasets you mounted, so isoshelf can see
your images but can't write to the folder.

isoshelf says so rather than failing silently. Open the app's **Logs** and you
will see lines naming the exact path and the exact problem — it carries on
with the built-in list of images rather than refusing to start, so a
permissions mistake never looks like a crash.

To fix it, make the two match:

- **Find the dataset's owner** (Datasets → your dataset → Permissions).
- **Put that UID and GID into the app's user and group fields**, or change the
  dataset's ownership to the user the app runs as.

On TrueNAS both are usually `568` (`apps`). The `/config` mount has to be
writable too, not just `/images` — that is where isoshelf keeps its settings
and the secret for your link.

### Checking it is alive

`http://your-truenas:8765/healthz` answers `ok` and needs no token. It says
nothing else — not which folder is open, not what is in it — so it is safe to
point a monitor at. The container's own health check uses it.

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

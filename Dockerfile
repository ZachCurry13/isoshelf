# isoshelf in a container, for a NAS or a home server: the page runs there and
# you open it from your own computer.
#
# Two folders matter, and both should be mounted from the host:
#   /images   the folder of bootable images to look after
#   /config   isoshelf's own files - settings, the catalog copy, the record of
#             what each folder held. Losing it loses that history, not images.
#
# It listens on 0.0.0.0 inside the container, which is what "server mode"
# means: the page may be opened by this machine's address rather than only by
# localhost. The secret in the link is then the only thing keeping anyone out.
# isoshelf makes that secret itself on first start and keeps it in /config, so
# a restart doesn't change it and there is nothing to set up; it prints the
# link in the log. Set ISOSHELF_TOKEN to choose the secret yourself.

FROM golang:1.27.1-alpine AS build
WORKDIR /src

# Dependencies first, so a change to the code doesn't refetch them.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=docker
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/isoshelf ./cmd/isoshelf

FROM alpine:3.21
# ca-certificates: isoshelf fetches over HTTPS and verifies what it downloads
# against the projects' published checksums, so it needs to trust their sites.
RUN apk add --no-cache ca-certificates tzdata

COPY --from=build /out/isoshelf /usr/local/bin/isoshelf

# isoshelf keeps its own files wherever the config folder points, and Go's
# os.UserConfigDir honours XDG_CONFIG_HOME. So /config is all it needs.
ENV XDG_CONFIG_HOME=/config

# Not root. 1000 is the common default; isoshelf needs no home directory and
# no passwd entry, so running it as any other uid works - TrueNAS runs its
# apps as 568, and simply overrides this. What matters is that whoever it
# runs as can write to the two folders. docs/docker.md explains that for
# someone who has not done it before.
RUN addgroup -g 1000 isoshelf && adduser -D -u 1000 -G isoshelf isoshelf

# The two folders have to exist here, and belong to that user, before VOLUME
# names them. Docker creates a missing volume path as root, and the container
# is not root, so without this a plain "docker run" with no mounts comes up
# unable to write its own settings. A bind mount takes the host's ownership
# instead, which is the way this is normally run and why the documentation
# spends a section on matching the two.
RUN mkdir -p /images /config && chown 1000:1000 /images /config && chmod 775 /images /config
VOLUME ["/images", "/config"]
EXPOSE 8765

USER 1000:1000

# /healthz is the one path that needs no token, and tells nothing about the
# folder or its contents.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s \
    CMD wget -q -O /dev/null http://127.0.0.1:8765/healthz || exit 1

ENTRYPOINT ["isoshelf", "ui", "--listen", "0.0.0.0", "--port", "8765", "--no-browser", "/images"]

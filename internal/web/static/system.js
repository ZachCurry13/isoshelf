"use strict";

// ---- Where this isoshelf runs ----------------------------------------------

// The facts under the version in Settings → isoshelf itself (v0.8.13): which
// system and how it was started, how long it has been running, and which list
// of images it is using. They were the Usage section's second half, and are
// what somebody asks for when reporting a bug.

const MODE = {
  desktop: "on this computer",
  portable: "portable, from the drive",
  server: "as a server on your network",
  container: "in a container",
};

const SYSTEM_NAME = { linux: "Linux", windows: "Windows", darwin: "macOS" };
const ARCH_NAME = { amd64: "x86-64", arm64: "ARM64", "386": "32-bit x86" };

// howLong writes a span of time the way a person would say it.
function howLong(ms) {
  const minutes = Math.floor(ms / 60000);
  if (minutes < 60) return plural(Math.max(minutes, 1), "minute");
  const hours = Math.floor(minutes / 60);
  if (hours < 48) return plural(hours, "hour");
  return plural(Math.floor(hours / 24), "day");
}

// systemFacts are the lines under the version, as text.
function systemFacts() {
  const sys = state && state.system;
  if (!sys) return [];
  const facts = [
    `${SYSTEM_NAME[sys.os] || sys.os} ${ARCH_NAME[sys.arch] || sys.arch}, ${MODE[sys.mode] || sys.mode}`,
  ];
  const started = new Date(sys.started);
  if (!isNaN(started)) {
    facts.push(`Running for ${howLong(Date.now() - started.getTime())}, since ${plainDateTime(started)}`);
  }
  if (sys.catalog_revision) facts.push(`Image list: revision ${sys.catalog_revision}`);
  return facts;
}

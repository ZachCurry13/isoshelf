"use strict";

// Installing an older version, and the hold that follows (#2). For when a new
// release breaks something: go back to the one before, and have isoshelf stop
// offering the broken one every time it checks. The versions are the ones the
// project still publishes (/api/versions); the download is checked against
// the project's own checksum like any other, and refused if there isn't one.
// A held image is treated as dismissed everywhere (dismiss.js): out of the
// updates count, Update all and updating by itself, until it is released or
// the newest is installed by hand.

// heldAt is the version the image is held at, or "".
function heldAt(entry) {
  const t = (entry && state.tracks && state.tracks[entry]) || {};
  return t.hold || "";
}

// holdField is the details panel's word on a hold, with Release.
function holdField(item) {
  return el("div", {},
    el("div", {}, `Held at ${heldAt(item.entry)}. Nothing updates this image, and its newer versions aren't offered, until you release it.`),
    el("button", { type: "button", class: "btn small", onclick: () => releaseHold(item.entry) }, "Release hold"));
}

async function releaseHold(entry) {
  try {
    await api("POST", "/api/track", { entry, release_hold: true });
  } catch (err) {
    showNotice(err.message, true);
    return;
  }
  await refresh();
}

// olderField asks the project for its versions only when somebody wants
// them: it is a request to the project's site, and most people never will.
function olderField(item) {
  const box = el("div", {});
  const show = el("button", { type: "button", class: "btn small" }, "Choose an older version");
  show.onclick = async () => {
    show.disabled = true;
    box.replaceChildren(el("div", { class: "muted", role: "status" }, "Asking the project which versions it publishes…"));
    let got;
    try {
      got = await api("GET", `/api/versions?entry=${encodeURIComponent(item.entry)}`);
    } catch (err) {
      box.replaceChildren(el("div", { class: "muted", role: "status" }, err.message), show);
      show.disabled = false;
      return;
    }
    box.replaceChildren(olderPicker(item, got));
  };
  box.append(show,
    el("div", { class: "muted" }, "For when a new release breaks something: go back to one the project still publishes."));
  return box;
}

// olderPicker lists the versions older than the newest, leaving out the one
// already here.
function olderPicker(item, got) {
  const older = got.versions.filter((v) => !v.newest && v.version !== item.version);
  if (got.why) return el("div", { class: "muted", role: "status" }, got.why);
  if (!older.length) {
    return el("div", { class: "muted", role: "status" },
      "The project publishes no older version of this image that isoshelf can find. Its own page may list more.");
  }
  const pick = el("select", { "aria-label": `Older version of ${item.name}` },
    older.map((v) => el("option", { value: v.version }, v.eol ? `${v.version} (end of life)` : v.version)));
  return el("div", { role: "status" },
    el("div", { class: "version-set" }, pick,
      el("button", { type: "button", class: "btn small", onclick: () => installOlder(item, pick.value) }, "Install")),
    el("div", { class: "muted" }, "Installing one holds the image there until you release it."));
}

async function installOlder(item, version) {
  const choice = isPinned(item) ? "keeps the pinned file" : CHOICE_WORD[choiceFor(item)];
  const go = await ask(
    `Install ${item.name} ${version}?`,
    `It's downloaded and checked against the project's own checksum like any update, and then ${choice}. ` +
    "isoshelf stops offering newer versions of this image until you release the hold. " +
    "If the project no longer publishes a checksum for it, nothing is downloaded.",
    [
      { label: "Install and hold", value: true, primary: true },
      { label: "Cancel", value: null },
    ]);
  if (!go) return;
  try {
    await api("POST", "/api/update", { entry: item.entry, removal: REMOVAL[choiceFor(item)], version });
  } catch (err) {
    showNotice(err.message, true);
  }
  await refresh();
}

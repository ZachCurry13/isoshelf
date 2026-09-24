"use strict";

// Doing something about an image: updating it, removing it, emptying the
// archive, bringing back one that was here, and the choices that follow one
// image around. The Add images tab is catalog.js; working out what a file
// is, identify.js.

// ---- Updating and removing -------------------------------------------------

// ask shows a dialog and returns the value of the button the user picked, or
// null if they closed it. body, when given, is shown under the text: a report
// needs to show what it would send, which is more than a sentence.
function ask(title, text, choices, body) {
  const dialog = $("ask");
  $("ask-title").textContent = title;
  $("ask-text").textContent = text;
  $("ask-text").hidden = !text;
  const extra = $("ask-body");
  extra.replaceChildren();
  extra.hidden = !body;
  if (body) extra.append(body);
  const buttons = $("ask-buttons");
  buttons.replaceChildren();
  return new Promise((resolve) => {
    for (const choice of choices) {
      buttons.append(el("button", {
        type: "button",
        class: `btn ${choice.primary ? "primary" : ""}`,
        onclick: () => { dialog.close(); resolve(choice.value); },
      }, choice.label));
    }
    dialog.addEventListener("close", () => resolve(null), { once: true });
    dialog.showModal();
  });
}

// REMOVAL turns an image's saved choice into what a download needs.
const REMOVAL = { replace: "delete", archive: "move-aside", keep: "keep" };

// Updating one image asks nothing: Settings says what happens to the copy it
// replaces (choiceFor), and a pinned file is kept whatever it says - the
// server sees to that.
async function updateItem(item) {
  await queueDownload(item.entry, REMOVAL[choiceFor(item)]);
}

// updatable lists the images with an update isoshelf can download.
function updatable() {
  return state.report.items.filter((it) => it.entry && it.updates === "download" && it.status === "update available");
}

// downloadSize is about how much an update to item downloads: the catalog's
// size for the image, else the size of the file already here. It was only
// ever the second, which is close on a real drive and nothing like it on a
// folder of stand-ins.
function downloadSize(item) {
  const entry = catalog && catalog.find((e) => e.id === item.entry);
  return (entry && entry.size) || item.size || 0;
}

// updateAll shows what it is about to do, image by image, and updates the
// ones still ticked.
async function updateAll() {
  const items = updatable().filter((it) => !inDownloads(it.entry));
  if (!items.length) return;
  const answer = await pickFiles({
    title: items.length === 1 ? "Update 1 image?" : `Update ${items.length} images?`,
    text: "Each one is downloaded and verified before anything is replaced. Untick any you would rather leave.",
    sizeLabel: "downloading about",
    rows: items.map((item) => ({
      id: item.entry,
      size: downloadSize(item),
      name: item.name,
      detail: `${item.version || "?"} → ${item.latest || "newest"}`,
      note: isPinned(item) ? "keeps this pinned file" : CHOICE_WORD[choiceFor(item)],
    })),
    actions: [{ label: "Update them", value: "go", primary: true }],
  });
  if (!answer) return;
  const chosen = new Set(answer.ids);
  for (const item of items.filter((it) => chosen.has(it.entry))) {
    try {
      await api("POST", "/api/update", { entry: item.entry, removal: REMOVAL[choiceFor(item)] });
    } catch (err) {
      showNotice(err.message, true);
    }
  }
  await refresh();
}

async function removeItem(item) {
  // Removing is the user's own choice, so a pin doesn't stop it - but it is
  // said, since a pin means somebody once wanted this file kept.
  const pinned = isPinned(item) ? "This file is pinned. " : "";
  const how = await ask(
    `Remove ${item.path}?`,
    `${pinned}This file uses ${formatBytes(item.size)}. Archiving keeps it in this folder, under Archive, where you can restore it or delete it later — the space is not freed until you do.`,
    [
      { label: "Archive it", value: "move-aside", primary: true },
      { label: "Delete it now", value: "delete" },
      { label: "Cancel", value: null },
    ]);
  if (!how) return;
  try {
    state = await api("POST", "/api/remove", { paths: [item.path], how });
    catalog = null;
  } catch (err) {
    showNotice(err.message, true);
    return;
  }
  if (how === "move-aside") {
    flashNotice(`Archived ${item.path}. It is under Archive on this page, where you can restore it; the space is freed when you empty the archive.`);
  } else {
    flashNotice(`Deleted ${item.path}.`);
  }
  render();
}

async function emptyRemoved() {
  const confirmed = await ask(
    "Empty the archive?",
    `${plural(state.removed.files, "file")} using ${formatBytes(state.removed.bytes)} will be deleted permanently. This frees the space.`,
    [{ label: "Delete them", value: "yes", primary: true }, { label: "Cancel", value: null }]);
  if (!confirmed) return;
  try {
    state = await api("POST", "/api/removed/empty");
    render();
  } catch (err) {
    showNotice(err.message, true);
  }
}

// ---- Images that were here -------------------------------------------------

const GONE_LABEL = {
  "removed": "deleted",
  "moved-aside": "moved aside",
  "replaced": "replaced by a newer file",
  "vanished": "gone from the folder",
};

async function restore(item) {
  try {
    await api("POST", "/api/restore", { name: item.path.split("/").pop() });
  } catch (err) {
    showNotice(err.message, true);
    return;
  }
  // A scan of its own would fight the one already running; that one, or the
  // one after the downloads, picks the file up.
  if (scanning()) {
    flashNotice(`Put ${item.path} back. It shows in the list once the scan finishes.`);
    await refresh();
    return;
  }
  await start("scan");
}

async function setTrack(entry, change) {
  try {
    const result = await api("POST", "/api/track", { entry, ...change });
    state.tracks = result.tracks;
    state.usual_set = result.usual_set;
  } catch (err) {
    showNotice(err.message, true);
  }
  renderRows();
}

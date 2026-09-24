"use strict";

// An image you usually keep here that isn't here (v0.7.1, #55): getting it
// back - from the archive if the file is still there, else by downloading
// it, else from its download page - or saying you removed it on purpose.

// archivedFor maps an image to its file still waiting in the archive, from
// the last time the archive was read. Putting that back is instant and gives
// back the very file that was there, so it comes before any download.
let archivedFor = {};
let archivedKey = "";

// noteArchive is told what the archive holds each time it is read. The rows
// are drawn before it arrives, so they are drawn again when it changes.
function noteArchive(items) {
  const next = {};
  for (const item of items || []) {
    if (item.entry && item.on_disk && item.restorable && !next[item.entry]) next[item.entry] = item;
  }
  archivedFor = next;
  const key = Object.keys(next).sort().join(",");
  if (key === archivedKey) return;
  archivedKey = key;
  renderRows();
  renderDetails();
}

// missingAction is the one way back a missing image offers, or null.
function missingAction(item, small) {
  const size = small ? "btn small" : "btn";
  const archived = archivedFor[item.entry];
  if (archived) {
    return el("button", {
      type: "button", class: `${size} primary`, disabled: scanning(),
      title: `${archived.path} is still in the archive. Put it back in this folder.`,
      onclick: () => restore(archived),
    }, "Restore");
  }
  if (item.updates === "download") {
    return jobButton(item.entry, el("button", {
      type: "button", class: `${size} primary`,
      title: "Download the newest version into this folder, checked like any other download",
      onclick: () => queueDownload(item.entry, "keep"),
    }, "Download again"), true);
  }
  if (item.page) {
    return el("a", {
      class: size, href: item.page, target: "_blank", rel: "noopener noreferrer",
      title: `isoshelf can't download ${item.name} for you. Its download page opens in a new tab.`,
    }, "Download page");
  }
  return null;
}

// expectField is "Stop expecting it", in the details panel of a missing
// image: for one removed on purpose. It is expected again once it is back.
function expectField(item) {
  return el("div", {},
    el("button", {
      type: "button", class: "btn small", disabled: scanning(),
      onclick: () => stopExpecting(item),
    }, "Stop expecting it"),
    el("div", { class: "muted" },
      "For an image you removed on purpose. It leaves the missing list, and " +
      "counts as usual again once it is back in this folder."));
}

async function stopExpecting(item) {
  try {
    await api("POST", "/api/track", { entry: item.entry, not_expected: true });
  } catch (err) {
    showNotice(err.message, true);
    return;
  }
  flashNotice(`isoshelf won't expect ${item.name} here until it is back in this folder.`);
  await refresh();
}

// downloadableMissing lists the missing images isoshelf could fetch again
// now: not already queued, and not waiting in the archive to be put back.
function downloadableMissing() {
  if (!state.report) return [];
  return state.report.items.filter((it) => it.status === "missing" && it.entry &&
    it.updates === "download" && !inDownloads(it.entry) && !archivedFor[it.entry]);
}

// downloadMissing is Download all, offered when the list shows the missing
// images: the same checklist as Update all, so each one can be left out.
async function downloadMissing() {
  const items = downloadableMissing();
  if (!items.length) return;
  const answer = await pickFiles({
    title: items.length === 1 ? "Download 1 image again?" : `Download ${items.length} images again?`,
    text: "Each one is the newest version, downloaded and verified before it goes in this folder. Untick any you don't want back.",
    sizeLabel: "downloading about",
    rows: items.map((item) => ({ id: item.entry, size: downloadSize(item), name: item.name, detail: item.latest || "" })),
    actions: [{ label: "Download them", value: "go", primary: true }],
  });
  if (!answer) return;
  for (const entry of answer.ids) {
    try {
      await api("POST", "/api/update", { entry, removal: "keep" });
    } catch (err) {
      showNotice(err.message, true);
    }
  }
  await refresh();
}

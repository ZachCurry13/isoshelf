"use strict";

// Two different things, kept apart on the page as well: the archive, which
// is files still on the drive that can be restored, and history, which is a
// record of images that have left.

// ---- Archive and history ---------------------------------------------------

// Two different things, so two sections: files still on the drive that you
// can restore, and a record of images that have left.
async function renderArchive() {
  const past = await archiveItems();
  if (!past) return;
  const here = past.filter((item) => item.on_disk);
  $("archive").hidden = here.length === 0;
  $("archive-count").textContent = here.length
    ? `${plural(here.length, "file")} · ${formatBytes(here.reduce((sum, i) => sum + (i.size || 0), 0))}`
    : "";
  $("archive-empty").disabled = scanning() || downloading();
  $("archive-list").replaceChildren(...here.map((item) => pastItem(item, true)));
}

async function renderHistory() {
  const past = await archiveItems();
  if (!past) return;
  const gone = past.filter((item) => !item.on_disk);
  $("history").hidden = gone.length === 0;
  $("jump-history").hidden = gone.length === 0;
  $("history-count").textContent = plural(gone.length, "image");
  $("history-list").replaceChildren(...gone.map((item) => pastItem(item, false)));
}

// archiveItems fetches the archive once per draw, for both sections.
let archiveCache = { at: 0, items: null };

async function archiveItems() {
  if (!state.target) return [];
  if (Date.now() - archiveCache.at < 1000 && archiveCache.items) return archiveCache.items;
  try {
    const items = (await api("GET", "/api/archive")).items;
    archiveCache = { at: Date.now(), items };
    noteArchive(items);
    return items;
  } catch {
    return null;
  }
}

function pastItem(item, onDisk) {
  const when = item.gone_at ? timeAgo(item.gone_at) : "";
  const parts = [GONE_LABEL[item.gone] || item.gone, when, formatBytes(item.size)];
  if (onDisk && !item.restorable) {
    // It's here and it's using room, but its name is taken by the file that
    // replaced it, so there is nowhere to put it back to yet.
    parts.push("another file has its name now — remove that one to restore this");
  }
  const detail = parts.filter(Boolean).join(" · ");
  const buttons = [];
  if (onDisk && item.restorable) {
    buttons.push(el("button", {
      type: "button", class: "btn small primary", disabled: scanning(),
      title: "Move it back into this folder",
      onclick: () => restore(item),
    }, "Restore"));
  }
  // A file still in the archive doesn't need downloading again: restoring it
  // is instant, costs nothing and gives back the very file that was there.
  if (item.downloadable && !onDisk) {
    buttons.push(jobButton(item.entry, el("button", {
      type: "button", class: "btn small",
      title: "Download the current version again",
      onclick: () => queueDownload(item.entry, "keep"),
    }, "Download again"), false));
  }
  if (item.page) {
    buttons.push(el("a", {
      class: "btn small", href: item.page, target: "_blank", rel: "noopener noreferrer",
      title: `Where ${item.name} is published`,
    }, "Download page"));
  }
  return el("li", {},
    logoTile(item),
    el("div", { class: "info" },
      el("div", {}, el("span", { class: "name" }, item.name),
        item.version ? el("span", { class: "arch" }, item.version) : null),
      el("div", { class: "file" }, item.path),
      el("div", { class: "kind" }, detail)),
    el("div", { class: "past-actions" }, buttons));
}

// renderJump keeps the bar at the top honest: how many images are in each
// section, and no link to a section that has nothing in it.
function renderJump() {
  const images = state.report ? state.report.items.length : 0;
  $("jump-images").textContent = images ? `Your images (${images})` : "Your images";
  const missing = catalog ? catalog.filter((e) => !e.on_target).length : 0;
  $("jump-more").textContent = missing ? `Add images (${missing})` : "Add images";
  const archived = state.removed ? state.removed.files : 0;
  $("jump-archive").hidden = !archived;
  $("jump-archive").textContent = archived ? `Archive (${archived})` : "Archive";
}

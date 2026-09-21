"use strict";

// Two different things, kept apart on the page as well: the archive, which
// is files still on the drive that can be put back, and history, which is a
// record of images that have left.

// ---- Archive and history ---------------------------------------------------

// Two different things, so two sections: files still on the drive that you
// can put back, and a record of images that have left.
async function renderArchive() {
  const past = await archiveItems();
  if (!past) return;
  const here = past.filter((item) => item.restorable);
  $("archive").hidden = here.length === 0;
  $("archive-count").textContent = here.length
    ? `${plural(here.length, "file")} · ${formatBytes(here.reduce((sum, i) => sum + (i.size || 0), 0))}`
    : "";
  $("archive-empty").disabled = Boolean(state.run);
  $("archive-list").replaceChildren(...here.map((item) => pastItem(item, true)));
}

async function renderHistory() {
  const past = await archiveItems();
  if (!past) return;
  const gone = past.filter((item) => !item.restorable);
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
    return items;
  } catch {
    return null;
  }
}

function pastItem(item, restorable) {
  const when = item.gone_at ? timeAgo(item.gone_at) : "";
  const detail = [GONE_LABEL[item.gone] || item.gone, when, formatBytes(item.size)].filter(Boolean).join(" · ");
  const buttons = [];
  if (restorable) {
    buttons.push(el("button", {
      type: "button", class: "btn small", disabled: scanning(),
      title: "Move it back into the folder",
      onclick: () => restore(item),
    }, "Put back"));
  }
  if (item.downloadable) {
    buttons.push(jobButton(item.entry, el("button", {
      type: "button", class: "btn small",
      title: "Download the current version again",
      onclick: () => queueDownload(item.entry, "keep"),
    }, "Download again"), false));
  }
  if (item.page) {
    buttons.push(el("a", { class: "btn small", href: item.page, target: "_blank", rel: "noopener noreferrer" }, "Page"));
  }
  return el("li", {},
    logoTile(item),
    el("div", { class: "info" },
      el("div", {}, el("span", { class: "name" }, item.name),
        item.version ? el("span", { class: "arch" }, item.version) : null),
      el("div", { class: "kind" }, `${item.path} · ${detail}`)),
    buttons);
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

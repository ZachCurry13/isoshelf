"use strict";

// Doing something about an image: updating it, removing it, emptying the
// archive, working out what an unrecognized file is, and the choices that
// follow one image around.

// ---- Updating and removing -------------------------------------------------

// ask shows a dialog and returns the value of the button the user picked, or
// null if they closed it.
function ask(title, text, choices) {
  const dialog = $("ask");
  $("ask-title").textContent = title;
  $("ask-text").textContent = text;
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

// Updating one image asks nothing: the image carries its own choice about
// what happens to the copy it replaces (see choiceField).
async function updateItem(item) {
  await queueDownload(item.entry, REMOVAL[choiceFor(item)]);
}

// updatable lists the images with an update isoshelf can download.
function updatable() {
  return state.report.items.filter((it) => it.entry && it.updates === "download" && it.status === "update available");
}

// updateAll shows what it is about to do, image by image, and updates the
// ones still ticked.
async function updateAll() {
  const items = updatable().filter((it) => !inDownloads(it.entry));
  if (!items.length) return;
  const answer = await pickFiles({
    title: items.length === 1 ? "Update 1 image?" : `Update ${items.length} images?`,
    text: "Each one is downloaded and checked before anything is replaced. Untick any you would rather leave.",
    sizeLabel: "downloading about",
    rows: items.map((item) => ({
      id: item.entry,
      size: item.size,
      name: item.name,
      detail: `${item.version || "?"} → ${item.latest || "newest"}`,
      note: CHOICE_WORD[choiceFor(item)],
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
  const how = await ask(
    `Remove ${item.path}?`,
    `This file uses ${formatBytes(item.size)}. Archiving keeps it in this folder, under “Images that were here”, where you can put it back or empty it later — the space isn't freed until you do.`,
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
    flashNotice(`Archived ${item.path}. You will find it under Archive on this page, where you can put it back; the space is freed when you empty the archive.`);
  } else {
    flashNotice(`Deleted ${item.path}.`);
  }
  render();
}

async function emptyRemoved() {
  const confirmed = await ask(
    "Empty the removed folder?",
    `${plural(state.removed.files, "file")} using ${formatBytes(state.removed.bytes)} will be deleted for good. This frees the space.`,
    [{ label: "Delete them", value: "yes", primary: true }, { label: "Cancel", value: null }]);
  if (!confirmed) return;
  try {
    state = await api("POST", "/api/removed/empty");
    render();
  } catch (err) {
    showNotice(err.message, true);
  }
}

const CATALOG_SOURCE = {
  "built-in": "the list isoshelf was built with",
  "downloaded": "kept up to date from the project",
  "yours": "your own catalog file",
};

// renderCatalogStatus shows where the list of known images comes from, and
// lets the user decide whether isoshelf keeps it current by itself.
function renderCatalogStatus(footer) {
  const cat = state.catalog;
  if (!cat || !cat.entries) return;
  const where = CATALOG_SOURCE[cat.source] || cat.source;
  const when = cat.source === "downloaded" && cat.updated_at ? `, checked ${timeAgo(cat.updated_at)}` : "";
  const mine = cat.mine ? `, plus ${plural(cat.mine, "image")} you named yourself` : "";
  const row = el("div", { class: "footer-row" },
    el("span", {}, `${plural(cat.entries, "image")} known: ${where}${when}${mine}.`),
    // The list changes separately from isoshelf, so it says when it last did
    // and where the changes are written down.
    cat.changed ? el("span", { class: "muted" },
      `List last changed ${new Date(cat.changed + "T12:00:00").toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric" })} · `,
      el("a", { href: cat.changes_url, target: "_blank", rel: "noopener noreferrer" }, "What's new")) : null);

  if (cat.can_auto) {
    row.append(
      el("label", { class: "check", title: "New images arrive without a new isoshelf. Only the project's own repository is ever fetched, and a list that doesn't pass every check is refused." },
        el("input", {
          type: "checkbox", checked: cat.auto,
          onchange: (e) => setCatalogAuto(e.target.checked),
        }), " Keep this list up to date"),
      el("button", {
        type: "button", class: "btn small",
        title: "Ask the project for a newer list right now",
        onclick: refreshCatalog,
      }, "Check now"));
  } else if (cat.source === "yours") {
    row.append(el("span", { class: "muted" }, "isoshelf never changes a catalog you wrote."));
  }
  if (cat.note) row.append(el("span", { class: "muted" }, cat.note));
  if (cat.error) row.append(el("span", { class: "muted" }, cat.error));
  footer.append(row);
}

async function setCatalogAuto(on) {
  try {
    state = await api("POST", "/api/settings", { catalog_auto: on });
    catalog = null;
    render();
  } catch (err) {
    showNotice(err.message, true);
  }
}

async function refreshCatalog() {
  try {
    const result = await api("POST", "/api/catalog/refresh");
    if (result.message) showNotice(result.message);
  } catch (err) {
    showNotice(err.message, true);
  }
  catalog = null;
  await refresh();
}

function renderFooter() {
  const footer = $("footer");
  footer.replaceChildren();
  renderCatalogStatus(footer);
  if (state.removed && state.removed.files > 0) {
    footer.append(el("div", { class: "footer-row" },
      `Removed files waiting in .isoshelf/removed: ${plural(state.removed.files, "file")} using ${formatBytes(state.removed.bytes)}.`,
      el("button", { type: "button", class: "btn small", disabled: Boolean(state.run), onclick: emptyRemoved }, "Empty it")));
  }
  if (!state.report) return;
  for (const trash of state.report.trash) {
    footer.append(el("div", {}, `Trash folder ${trash.path} uses ${formatBytes(trash.bytes)}. Emptying it frees that space.`));
  }
  for (const problem of state.report.problems) {
    footer.append(el("div", {}, `Couldn't read ${problem.path}: ${problem.error}`));
  }
}

async function renderCatalog() {
  if (!state.report) {
    $("more-count").textContent = "";
    $("catalog").replaceChildren();
    return;
  }
  if (!catalog) {
    try {
      catalog = (await api("GET", "/api/catalog")).entries;
    } catch {
      return;
    }
  }
  const missing = catalog.filter((e) => !e.on_target);
  $("more-count").textContent = plural(missing.length, "image");
  // Images added from here stay listed, ticked, until the downloads are
  // cleared, so the button that was clicked says how it went.
  const listed = catalog.filter((e) => !e.on_target || (jobFor(e.id) || {}).where === "done");

  const shown = filterCatalog(listed);
  const list = $("catalog");
  list.replaceChildren();
  for (const entry of shown) {
    const room = jobFor(entry.id) ? null : fitsHere(entry);
    list.append(el("li", {},
      logoTile(entry),
      el("div", { class: "info" },
        el("div", { class: "info-line" }, el("span", { class: "name" }, entry.name)),
        el("div", { class: "meta-line" },
          el("span", { class: "arch" }, entry.arch),
          entry.popular ? el("span", { class: "pill s-ok", title: "Turns up in public round-ups of what people are running. A hand-picked hint, not a rating." }, "popular") : null,
          entry.size ? el("span", { class: "muted" }, `about ${formatBytes(entry.size)}`) : null),
        el("div", { class: "kind" },
          UPDATES_LABEL[entry.updates] || entry.updates,
          room === false
            ? el("span", { class: "wont-fit" }, fitsHere(entry, true) ? " · no room once the waiting downloads are in" : " · bigger than the room left here")
            : null)),
      entry.page ? el("a", { class: "btn small", href: entry.page, target: "_blank", rel: "noopener noreferrer" }, "Page") : null,
      addButton(entry, room)));
  }
  $("more-shown").textContent = shown.length === listed.length
    ? ""
    : `showing ${shown.length} of ${listed.length}`;

  // Whatever isoshelf knows, someone's favourite image won't be in it.
  const request = $("catalog-request");
  request.replaceChildren();
  if (state.report_url) {
    request.append(
      "Looking for an image that isn't listed? ",
      el("a", {
        href: `${state.report_url}?template=missing-image.yml`,
        target: "_blank", rel: "noopener noreferrer",
      }, "Ask for it to be added"),
      " — the form asks where the project publishes its checksums, which is the part that decides whether isoshelf can download it or only link to it.");
  }
  if (!shown.length) {
    list.append(el("li", { class: "muted more-hint" }, missing.length
      ? "None of these match the filters."
      : "Everything isoshelf knows about is already in this folder."));
  }
}

// fitsHere reports whether an image would fit in the folder once the
// downloads already waiting are in: true, false, or null when either the size
// or the free space is unknown. Without the queue, it asks about the room
// there is now.
function fitsHere(entry, withoutQueue) {
  if (!entry.size || !state.space || !state.space.total) return null;
  const spare = Math.min(state.space.total / 100, 1 << 30);
  const queued = withoutQueue ? 0 : pendingBytes();
  return state.space.free - queued - entry.size >= spare;
}

// filterCatalog applies the catalog list's own filters and sort. They are
// separate from the main list's, because the two lists are read for different
// reasons: what have I got, and what could I add.
function filterCatalog(entries) {
  const query = $("more-search").value.trim().toLowerCase();
  const category = $("more-category").value;
  const arch = $("more-arch").value;
  const updates = $("more-updates").value;
  const fitsOnly = $("more-fits").checked;
  const popularOnly = $("more-popular").checked;

  const shown = entries.filter((entry) => {
    if (query && !`${entry.name} ${entry.id} ${entry.family || ""}`.toLowerCase().includes(query)) return false;
    if (category && (entry.category || "other") !== category) return false;
    if (arch && entry.arch !== arch) return false;
    if (updates && entry.updates !== updates) return false;
    if (fitsOnly && fitsHere(entry) === false) return false;
    if (popularOnly && !entry.popular) return false;
    return true;
  });

  const byName = (a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
  switch ($("more-sort").value) {
    case "size":
      shown.sort((a, b) => (b.size || 0) - (a.size || 0) || byName(a, b));
      break;
    case "smallest":
      // Images with no size go last either way: an unknown size is not small.
      shown.sort((a, b) => (a.size || Infinity) - (b.size || Infinity) || byName(a, b));
      break;
    case "kind":
      shown.sort((a, b) => (a.category || "other").localeCompare(b.category || "other") || byName(a, b));
      break;
    case "popular":
      shown.sort((a, b) => Number(Boolean(b.popular)) - Number(Boolean(a.popular)) || byName(a, b));
      break;
    default:
      shown.sort(byName);
  }
  return shown;
}

// addButton downloads a catalog image this folder doesn't have yet. isoshelf
// can only do that for images whose checksums it can reach; for the rest the
// download page is the way.
function addButton(entry, room) {
  if (entry.updates === "download" && room === false) {
    const queued = pendingBytes();
    return jobButton(entry.id, el("button", {
      type: "button", class: "btn small", disabled: true,
      title: `${entry.name} is about ${formatBytes(entry.size)}, and this folder has ${formatBytes(state.space.free)} left${queued ? `, with about ${formatBytes(queued)} of downloads still to come` : ""}.`,
    }, "Add"), false);
  }
  if (entry.updates !== "download") {
    return el("button", {
      type: "button", class: "btn small", disabled: true,
      title: entry.page
        ? `isoshelf can't download ${entry.name} itself, because there is nowhere to check it against. Use its download page.`
        : `isoshelf can't download ${entry.name} itself: there is nowhere to check it against.`,
    }, "Add");
  }
  return jobButton(entry.id, el("button", {
    type: "button", class: "btn small primary",
    title: `Download the newest ${entry.name} into this folder. If something is downloading already, it waits its turn.`,
    onclick: () => queueDownload(entry.id, "keep"),
  }, "Add"), false);
}

// ---- What is this file? ----------------------------------------------------

// How sure isoshelf is, in words. A score above 80 rests on evidence: the
// same checksum, or the same file under another name.
const SURENESS = [[95, "Almost certain"], [80, "Very likely"], [60, "Likely"], [45, "Possible"], [0, "A guess"]];

function sureness(score) {
  for (const [least, label] of SURENESS) {
    if (score >= least) return label;
  }
  return "A guess";
}

// identifyPath is the file the dialog is about, so a slow answer for one file
// never lands in the dialog for another.
let identifyPath = null;
let identifyItem = null;

// suggestedName turns a filename into a first guess at a title, so the user
// edits rather than types: "WinServer_2022_x64.iso" -> "WinServer 2022".
function suggestedName(path) {
  const file = path.split("/").pop().replace(/\.[a-z0-9]{1,4}$/i, "");
  return file
    .replace(/[_.+]+/g, " ")
    .replace(/\b(x86[-_]?64|amd64|x64|i386|i686|arm64|aarch64|iso|img)\b/gi, "")
    .replace(/\s{2,}/g, " ")
    .trim()
    .slice(0, 80);
}

// renderReportLink offers to tell the project about an image its catalog is
// missing. It opens a prefilled report the user reads and sends themselves;
// isoshelf sends nothing on its own.
function renderReportLink(item, label) {
  const box = $("identify-report");
  box.replaceChildren();
  if (!state.report_url) return;
  const title = `Catalog: ${item.path.split("/").pop()}`;
  const body = [
    "An image isoshelf didn't recognize.",
    "",
    `- File: ${item.path.split("/").pop()}`,
    `- Size: ${formatBytes(item.size)}`,
    item.kind ? `- Content: ${item.kind}` : null,
    label ? `- The disc calls itself: ` : null,
    "",
    "What is it, and where is it published?",
  ].filter(Boolean).join("\n");
  const url = `${state.report_url}?title=${encodeURIComponent(title)}&body=${encodeURIComponent(body)}&labels=catalog`;

  box.append(
    "Should isoshelf know this image? ",
    el("a", { href: url, target: "_blank", rel: "noopener noreferrer" }, "Tell the project about it"),
    " — it opens a report you can read and change before sending. Nothing is sent by isoshelf.");
}

// saveMyName writes the user's own name for a file into their own catalog.
async function saveMyName() {
  const name = $("mine-name").value.trim();
  if (!name) {
    showNotice("Give the image a name first.", true);
    return;
  }
  let result;
  try {
    result = await api("POST", "/api/catalog/mine", {
      path: identifyPath,
      name,
      arch: $("mine-arch").value,
      category: $("mine-category").value,
      page: $("mine-page").value.trim(),
    });
  } catch (err) {
    showNotice(err.message, true);
    return;
  }
  $("identify").close();
  showNotice(result.message);
  catalog = null;
  await refresh();
}

async function openIdentify(item) {
  identifyPath = item.path;
  identifyItem = item;
  $("identify-file").textContent = item.path;
  $("identify-forget").hidden = !item.assigned;
  $("identify-search").value = "";
  $("identify-list").replaceChildren();
  $("identify-all").open = false;
  $("identify-mine").open = false;
  $("mine-name").value = suggestedName(item.path);
  $("mine-page").value = "";
  $("identify-report").replaceChildren();
  $("identify-guesses").replaceChildren(el("p", { class: "muted" }, "Looking at what the file says about itself…"));
  $("identify").showModal();

  let data;
  try {
    data = await api("GET", `/api/guesses?path=${encodeURIComponent(item.path)}`);
  } catch (err) {
    $("identify-guesses").replaceChildren(el("p", { class: "muted" }, err.message));
    return;
  }
  if (data.path !== identifyPath) return;
  renderGuesses(data.guesses);
  renderIdentifyCatalog();
  renderReportLink(item, data.label);
}

function renderGuesses(guesses) {
  const box = $("identify-guesses");
  box.replaceChildren();
  if (!guesses.length) {
    box.append(el("p", { class: "muted" },
      "isoshelf can't work this one out: nothing in the catalog looks like it, and no copy of it is in this folder. You can pick what it is yourself."));
    $("identify-all").open = true;
    return;
  }
  box.append(el("p", { class: "muted" },
    guesses.length === 1 ? "isoshelf thinks this might be:" : "isoshelf thinks this might be one of these:"));

  const list = el("ul", { class: "guesses" });
  for (const guess of guesses) {
    list.append(el("li", {},
      logoTile(guess),
      el("div", { class: "info" },
        el("div", {},
          el("span", { class: "name" }, guess.name),
          guess.arch ? el("span", { class: "arch" }, guess.arch) : null,
          guess.version ? el("span", { class: "arch" }, guess.version) : null,
          el("span", { class: `pill ${guess.sure ? "s-ok" : "s-muted"}` }, sureness(guess.score))),
        el("div", { class: "kind" }, `Because ${guess.reason}.`)),
      el("button", {
        type: "button", class: "btn small primary",
        onclick: () => confirmIdentity(guess.entry, guess.version),
      }, "That's it")));
  }
  box.append(list);
}

// renderIdentifyCatalog lists the catalog, so a file isoshelf can't place can
// still be identified by hand.
async function renderIdentifyCatalog() {
  if (!catalog) {
    try {
      catalog = (await api("GET", "/api/catalog")).entries;
    } catch {
      return;
    }
  }
  const query = $("identify-search").value.trim().toLowerCase();
  const list = $("identify-list");
  list.replaceChildren();
  let shown = 0;
  for (const entry of catalog) {
    if (query && !`${entry.name} ${entry.id} ${entry.family || ""}`.toLowerCase().includes(query)) continue;
    if (++shown > 40) {
      list.append(el("li", { class: "muted more-hint" }, "More images match. Keep typing to narrow it down."));
      break;
    }
    list.append(el("li", {},
      logoTile(entry),
      el("div", { class: "info" },
        el("div", {}, el("span", { class: "name" }, entry.name), el("span", { class: "arch" }, entry.arch)),
        el("div", { class: "kind" }, UPDATES_LABEL[entry.updates] || entry.updates)),
      el("button", {
        type: "button", class: "btn small",
        onclick: () => confirmIdentity(entry.id, ""),
      }, "This one")));
  }
  if (!shown) list.append(el("li", { class: "muted more-hint" }, "Nothing in the catalog matches that."));
}

// confirmIdentity records the user's answer. An empty entry forgets an
// earlier one. Nothing on disk is renamed or moved.
async function confirmIdentity(entry, version) {
  let result;
  try {
    result = await api("POST", "/api/identify", { path: identifyPath, entry, version: version || "" });
  } catch (err) {
    showNotice(err.message, true);
    return;
  }
  $("identify").close();
  showNotice(result.message);
  // The file's version is known now but not what the newest one is, so pick
  // the online check up again if it had already run.
  if (result.recheck) {
    await start("check");
  } else {
    await refresh();
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
  // A scan waits for downloads; the one that follows them picks the file up.
  if (state.run && state.run.kind === "update") {
    flashNotice(`Put ${item.path} back. It shows in the list once the downloads finish.`);
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

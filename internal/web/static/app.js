"use strict";

// The page itself: what isoshelf has told it, how it asks, and what gets
// drawn when the answer changes. Each part of the page has its own file
// beside this one - the list (images.js), downloads (downloads.js), updating
// and identifying (actions.js), the folder chooser (folders.js), the archive
// and history (archive.js) and Settings (settings.js). They are plain
// scripts sharing the same names, loaded in the order index.html lists them.

// Statuses in the order the report sorts them, with their color class.
const STATUSES = [
  ["checksum mismatch", "s-bad"],
  ["update available", "s-update"],
  ["EOL", "s-eol"],
  ["not bootable", "s-warn"],
  ["missing", "s-missing"],
  ["unrecognized", "s-muted"],
  ["check failed", "s-bad"],
  ["unknown", "s-muted"],
  ["not checked", "s-muted"],
  ["manual", "s-manual"],
  ["up to date", "s-ok"],
];
const STATUS_CLASS = Object.fromEntries(STATUSES);

const UPDATES_LABEL = {
  "download": "Update checks and downloads",
  "check-only": "Update checks",
  "manual": "Download page link",
};

const $ = (id) => document.getElementById(id);

let state = null;
let catalog = null;
let statusFilter = null;
let pollTimer = null;
let autoScanned = false;
// drawn is drawnKey of the state the page last drew in full; lastScan the
// scan time the catalog was fetched for.
let drawn = "";
let lastScan;

// What the list shows, kept in the browser between visits. Filters live in
// one menu and show up as chips, so it is always clear why the list is short.
const view = {
  sort: "attention",
  desc: false,
  show: { updates: false, favorites: false, older: false, caution: false },
  kinds: [],
  arches: [],
  status: "",
};

function loadView() {
  let saved = {};
  try {
    saved = JSON.parse(localStorage.getItem("isoshelf.view") || "{}");
  } catch {
    // A browser that will not remember settings is fine; the defaults apply.
  }
  Object.assign(view, saved, { show: { ...view.show, ...(saved.show || {}) } });
  // Older versions kept each filter on its own; carry those over once.
  if (saved.category) view.kinds = [saved.category];
  if (saved.arch) view.arches = [saved.arch];
  if (saved.updatesOnly) view.show.updates = true;
  if (saved.favoritesOnly) view.show.favorites = true;
  if (saved.olderOnly) view.show.older = true;
  // "Recently changed" became "Recently added": a copied file keeps its old
  // change date, so it never answered the question people were asking.
  if (view.sort === "modified") view.sort = "added";
  delete view.category;
  delete view.arch;
  delete view.updatesOnly;
  delete view.favoritesOnly;
  delete view.olderOnly;
  $("sort").value = view.sort;
}

function saveView() {
  try {
    localStorage.setItem("isoshelf.view", JSON.stringify(view));
  } catch {
    // Not being able to remember the choice does not matter.
  }
}

// ---- Talking to isoshelf -------------------------------------------------

async function api(method, path, body) {
  const options = { method, headers: { "X-Isoshelf": "1" } };
  if (body !== undefined) {
    options.headers["Content-Type"] = "application/json";
    options.body = JSON.stringify(body);
  }
  let response;
  try {
    response = await fetch(path, options);
  } catch {
    throw new Error("isoshelf isn't responding. Is it still running?");
  }
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(data.error || `${response.status} ${response.statusText}`);
  }
  return data;
}

async function refresh() {
  try {
    state = await api("GET", "/api/state");
  } catch (err) {
    showNotice(err.message, true);
    schedule(5000);
    return;
  }
  // A new scan changes which catalog images are already here.
  if (state.updated_at !== lastScan) {
    lastScan = state.updated_at;
    catalog = null;
  }
  // A mistake while drawing must never stop the page from updating.
  try {
    // Downloads can run for an hour, and the page asks how they're doing
    // twice a second. Redrawing everything each time would close open menus
    // and lose the button that had focus, so only the moving parts move
    // unless something else changed.
    if (drawnKey(state) === drawn) renderMoving();
    else render();
  } catch (err) {
    showNotice(`Something went wrong while drawing the page: ${err.message}`, true);
  }
  if (state.run) {
    schedule(600);
    return;
  }
  if (state.target && !state.report && !state.error && !autoScanned) {
    autoScanned = true;
    start("scan");
  } else if (!state.target && !$("picker").open) {
    openPicker();
  }
}

function schedule(ms) {
  clearTimeout(pollTimer);
  pollTimer = setTimeout(refresh, ms);
}

async function start(kind) {
  try {
    await api("POST", `/api/${kind}`);
    catalog = null;
  } catch (err) {
    showNotice(err.message, true);
  }
  refresh();
}

// ---- Rendering -----------------------------------------------------------

function el(tag, attrs = {}, ...children) {
  const node = document.createElement(tag);
  for (const [key, value] of Object.entries(attrs)) {
    if (value === undefined || value === null || value === false) continue;
    if (key === "class") node.className = value;
    else if (key.startsWith("on")) node.addEventListener(key.slice(2), value);
    else node.setAttribute(key, value === true ? "" : value);
  }
  for (const child of children.flat()) {
    if (child === null || child === undefined) continue;
    node.append(child instanceof Node ? child : document.createTextNode(String(child)));
  }
  return node;
}

// drawnKey is the state minus the parts that move on their own: progress and
// free space. When it hasn't changed, renderMoving is enough.
function drawnKey(s) {
  const { run, space, ...rest } = s;
  return JSON.stringify([rest, run ? run.kind : null]);
}

// renderMoving updates only what changes while something runs.
function renderMoving() {
  renderSpace();
  renderRun();
  renderDockProgress();
}

// freshness says when the list was last brought up to date. Scanning and
// checking are two different questions - what is in the folder, and what the
// projects have published - and the answer to the second can be older,
// because an answer from earlier today is used as it stands.
function freshness() {
  if (!state.updated_at) return "";
  if (!state.report || !state.report.checked) return `Scanned ${timeAgo(state.updated_at)}`;
  return `Checked ${timeAgo(state.checked_at || state.updated_at)}`;
}

function renderSpace() {
  $("space").textContent = state.space && state.space.total
    ? `${formatBytes(state.space.free)} free of ${formatBytes(state.space.total)}`
    : "";
}

// scanning is true while a scan or check runs. Downloads are different: the
// page stays usable while they run, and more can be added to the queue.
function scanning() {
  return Boolean(state.run && state.run.kind !== "update");
}

function render() {
  drawn = drawnKey(state);
  const busy = Boolean(state.run);
  $("version").textContent = state.version;
  // The look is settings.js's business, and so is the panel itself.
  applyAppearance(state.appearance);
  renderSettings();

  const update = $("app-update");
  update.hidden = !state.app_update;
  if (state.app_update) {
    update.textContent = `isoshelf ${state.app_update.latest} is available`;
    update.href = state.app_update.url;
  }

  $("target-path").textContent = state.target || "No folder chosen yet";
  $("profile").value = state.profile || "ventoy";
  $("profile").disabled = busy || !state.target;
  $("choose").disabled = busy;
  $("refresh").disabled = busy || !state.target;
  $("updated-at").textContent = freshness();
  renderSpace();
  renderRun();
  renderDock();

  if (state.error) showNotice(state.error, true);
  else if (flash && Date.now() < flash.until) showNotice(flash.message, false);
  else if (state.warnings.length) showNotice(state.warnings.join(" "), false);
  else $("notice").hidden = true;

  renderJump();
  renderTodo();
  renderFilters();
  renderHeadings();
  renderRows();
  renderFooter();
  renderCatalog();
  renderArchive();
  renderHistory();
  renderDetails();
}

function renderRun() {
  const run = state.run;
  // Downloads have their own bar at the bottom of the page.
  $("run").hidden = !run || run.kind === "update";
  if (!run || run.kind === "update") return;
  let text = "Scanning the folder…";
  let fraction = null;
  if (run.stage === "hashing") {
    // Only images whose filename never changes need this, and only once
    // each, but on a USB drive it is minutes of reading.
    const which = run.items > 1 ? ` (${run.item} of ${run.items})` : "";
    text = `Checking what ${run.file} is${which} — this happens once per image`;
    if (run.total > 0) fraction = run.done / run.total;
  } else if (run.stage === "checking") {
    text = "Checking for updates…";
    if (run.total > 0) {
      text = `Checking for updates (${run.done} of ${run.total})…`;
      fraction = run.done / run.total;
    }
  } else if (run.stage === "saving") {
    text = "Saving…";
  }
  $("run-text").textContent = text;
  $("run-bar").classList.toggle("indeterminate", fraction === null);
  $("run-fill").style.width = fraction === null ? "" : `${Math.round(fraction * 100)}%`;
}
// ---- Helpers ---------------------------------------------------------------

// plural writes "1 image" but "2 images".
function plural(count, word, plural) {
  if (count === 1) return `${count} ${word}`;
  return `${count} ${plural || word + "s"}`;
}

function formatBytes(bytes) {
  if (!bytes) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  while (bytes >= 1024 && i < units.length - 1) { bytes /= 1024; i++; }
  return `${bytes.toFixed(i === 0 || bytes >= 100 ? 0 : 1)} ${units[i]}`;
}

function timeAgo(iso) {
  const seconds = Math.round((Date.now() - new Date(iso).getTime()) / 1000);
  if (seconds < 60) return "just now";
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes} min ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours} h ago`;
  return new Date(iso).toLocaleString();
}

// ---- Wiring ----------------------------------------------------------------

document.addEventListener("DOMContentLoaded", () => {
  // An open menu is pinned to the window; scrolling would leave it behind.
  document.addEventListener("scroll", closeMenus, true);
  window.addEventListener("resize", closeMenus);
  $("choose").addEventListener("click", openPicker);
  $("refresh").addEventListener("click", () => start("check"));
  $("cancel").addEventListener("click", async () => {
    try { await api("POST", "/api/cancel"); } catch (err) { showNotice(err.message, true); }
  });
  $("profile").addEventListener("change", async (e) => {
    try {
      await api("POST", "/api/target", { path: state.target, profile: e.target.value });
      catalog = null;
      await start("scan");
    } catch (err) {
      showNotice(err.message, true);
    }
  });
  $("dock-toggle").addEventListener("click", () => {
    dockOpen = !dockOpen;
    renderDock();
  });
  $("dock-stop").addEventListener("click", stopDownloads);
  $("dock-clear").addEventListener("click", clearFinished);
  document.addEventListener("keydown", (e) => {
    if (e.key !== "Escape" || document.querySelector("dialog[open]")) return;
    if (settingsOpen) {
      closeSettings();
    } else if (detailsOpen) {
      closeDetails();
    } else if (dockOpen) {
      dockOpen = false;
      renderDock();
    }
  });
  $("search").addEventListener("input", () => { renderChips(); renderRows(); });
  $("details-close").addEventListener("click", closeDetails);
  $("what-mean").addEventListener("click", showMeanings);
  $("archive-empty").addEventListener("click", emptyRemoved);
  loadView();
  // Picking a sort from the list means the way that option is worded:
  // "largest first" is already the right way round.
  $("sort").addEventListener("change", (e) => {
    view.sort = e.target.value;
    view.desc = false;
    saveView();
    renderHeadings();
    renderRows();
  });
  $("more-search").addEventListener("input", renderCatalog);
  for (const id of ["more-category", "more-arch", "more-sort", "more-updates", "more-fits", "more-popular"]) {
    $(id).addEventListener("change", renderCatalog);
  }
  $("identify-search").addEventListener("input", renderIdentifyCatalog);
  $("identify-forget").addEventListener("click", () => confirmIdentity("", ""));
  $("mine-save").addEventListener("click", saveMyName);
  $("picker-go").addEventListener("click", () => browse($("picker-input").value.trim()));
  $("picker-input").addEventListener("keydown", (e) => {
    if (e.key === "Enter") { e.preventDefault(); browse($("picker-input").value.trim()); }
  });
  $("picker-use").addEventListener("click", useFolder);
  $("picker-refresh").addEventListener("click", () => browse(pickerPath));
  refresh();
  // Keep "checked 5 min ago" fresh.
  setInterval(() => { if (state && !state.run) render(); }, 60000);
});

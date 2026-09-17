"use strict";

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
  render();
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

function render() {
  const busy = Boolean(state.run);
  $("version").textContent = state.version;

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
  $("scan").disabled = busy || !state.target;
  $("check").disabled = busy || !state.target;
  $("updated-at").textContent = state.updated_at
    ? `${state.report && state.report.checked ? "Checked" : "Scanned"} ${timeAgo(state.updated_at)}`
    : "";

  renderRun();

  if (state.error) showNotice(state.error, true);
  else if (state.warnings.length) showNotice(state.warnings.join(" "), false);
  else $("notice").hidden = true;

  renderSummary();
  renderRows();
  renderFooter();
  renderCatalog();
}

function renderRun() {
  const run = state.run;
  $("run").hidden = !run;
  if (!run) return;
  let text = "Scanning the folder…";
  let fraction = null;
  if (run.stage === "hashing") {
    text = `Hashing ${run.file}`;
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

function showNotice(message, isError) {
  const notice = $("notice");
  notice.textContent = message;
  notice.classList.toggle("error", isError);
  notice.hidden = false;
}

function renderSummary() {
  const summary = $("summary");
  summary.replaceChildren();
  if (!state.report) return;
  const counts = {};
  for (const item of state.report.items) counts[item.status] = (counts[item.status] || 0) + 1;
  if (statusFilter && !counts[statusFilter]) statusFilter = null;

  for (const [status, cls] of STATUSES) {
    if (!counts[status]) continue;
    summary.append(el("button", {
      type: "button",
      class: `chip ${cls}`,
      "aria-pressed": statusFilter === status ? "true" : "false",
      onclick: () => { statusFilter = statusFilter === status ? null : status; renderSummary(); renderRows(); },
    }, el("span", { class: "dot" }), el("span", { class: "count" }, counts[status]), status));
  }
}

function renderRows() {
  const rows = $("rows");
  rows.replaceChildren();
  const empty = $("empty");
  if (!state.report) {
    empty.hidden = false;
    empty.textContent = state.target
      ? "Scanning will list the images in this folder."
      : "Choose a folder to see its images.";
    $("shown").textContent = "";
    return;
  }

  const query = $("search").value.trim().toLowerCase();
  const items = state.report.items.filter((item) =>
    (!statusFilter || item.status === statusFilter) &&
    (!query || `${item.name} ${item.path || ""} ${item.entry || ""}`.toLowerCase().includes(query)));

  for (const item of items) rows.append(renderRow(item));

  const total = state.report.items.length;
  $("shown").textContent = items.length === total ? `${total} images` : `${items.length} of ${total} images`;
  empty.hidden = items.length > 0;
  empty.textContent = total === 0 ? "No images found in this folder." : "Nothing matches the filter.";
}

function renderRow(item) {
  const track = (item.entry && state.tracks[item.entry]) || {};
  const busy = Boolean(state.run);
  const usual = item.entry && state.usual_set.includes(item.entry);

  // A filled star means the user starred the image. Images that are only in
  // the usual set because recent scans saw them keep an outline star.
  const star = el("button", {
    type: "button",
    class: "star",
    "aria-pressed": track.starred ? "true" : "false",
    title: track.starred
      ? "Starred: isoshelf reports it as missing if it ever disappears from this folder. Click to unstar."
      : usual
        ? "Usually kept here (seen in recent scans). Star it to always report it if it goes missing."
        : "Star to always report this image if it goes missing from this folder",
    "aria-label": `Star ${item.name}`,
    disabled: !item.entry || busy,
    onclick: () => setTrack(item.entry, { starred: !track.starred }),
  }, track.starred ? "★" : "☆");

  const statusCell = el("td", {},
    el("span", { class: `pill ${STATUS_CLASS[item.status] || "s-muted"}` }, item.status),
    item.eol && item.status !== "EOL" ? el("span", { class: "pill s-eol", title: "This release is no longer supported" }, "EOL") : null);

  const name = item.page
    ? el("a", { href: item.page, target: "_blank", rel: "noopener noreferrer", title: "Open the download page" }, item.name)
    : item.name;
  const imageCell = el("td", {},
    el("div", {}, el("span", { class: "name" }, name), item.arch ? el("span", { class: "arch" }, item.arch) : null),
    item.note ? el("div", { class: "note" }, item.note) : null);

  const newer = item.latest && item.status === "update available";
  const latestCell = el("td", { class: "latest-cell" },
    item.latest ? el("span", { class: newer ? "latest-new" : "", title: item.latest_file || "" }, item.latest) : "–");

  const fileCell = el("td", {},
    item.path
      ? [el("div", { class: "file" }, item.path), el("div", { class: "size" }, formatBytes(item.size), item.kind && item.kind !== "unknown" ? ` · ${item.kind}` : "")]
      : el("span", { class: "muted" }, "Not in this folder"));

  let replace = el("span", { class: "muted", title: "Nothing to download for this image" }, "–");
  if (item.entry && item.updates === "download") {
    const input = el("input", {
      type: "checkbox",
      "aria-label": `Replace old ${item.name} files after an update`,
      checked: !track.keep_old,
      disabled: busy,
      onchange: (e) => setTrack(item.entry, { keep_old: !e.target.checked }),
    });
    replace = el("label", { class: "switch", title: "On: replace the old file after a verified update. Off: keep both." }, input, el("span", { class: "track" }));
  }

  return el("tr", {},
    el("td", { class: "col-star" }, star),
    statusCell,
    imageCell,
    el("td", { class: "version-cell" }, item.version || "–"),
    latestCell,
    fileCell,
    el("td", { class: "replace" }, replace));
}

function renderFooter() {
  const footer = $("footer");
  footer.replaceChildren();
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
  $("more-count").textContent = `${missing.length} images`;
  const query = $("more-search").value.trim().toLowerCase();
  const list = $("catalog");
  list.replaceChildren();
  for (const entry of missing) {
    if (query && !`${entry.name} ${entry.id}`.toLowerCase().includes(query)) continue;
    list.append(el("li", {},
      el("div", { class: "info" },
        el("div", {}, el("span", { class: "name" }, entry.name), el("span", { class: "arch" }, entry.arch)),
        el("div", { class: "kind" }, UPDATES_LABEL[entry.updates] || entry.updates)),
      entry.page ? el("a", { class: "btn small", href: entry.page, target: "_blank", rel: "noopener noreferrer" }, "Page") : null,
      el("button", { type: "button", class: "btn small", disabled: true, title: "Adding images arrives with downloads in v0.2" }, "Add")));
  }
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

// ---- Folder picker ---------------------------------------------------------

let pickerPath = "";

async function openPicker() {
  const dialog = $("picker");
  if (!dialog.open) dialog.showModal();
  await browse(state && state.target ? state.target : "");
}

async function browse(path) {
  let result;
  try {
    result = await api("GET", `/api/browse?path=${encodeURIComponent(path)}`);
  } catch (err) {
    showPickerError(err.message);
    return;
  }
  pickerPath = result.path || "";
  $("picker-input").value = pickerPath;
  $("picker-up").disabled = !result.parent;
  $("picker-up").onclick = () => browse(result.parent);
  $("picker-use").disabled = !pickerPath || Boolean(result.error);
  if (result.suggested_profile) $("picker-profile").value = result.suggested_profile;
  showPickerError(result.error);

  const roots = $("picker-roots");
  roots.replaceChildren();
  if (state && state.recent_targets.length) {
    roots.append(el("h3", {}, "Recent"));
    for (const target of state.recent_targets) {
      roots.append(el("button", { type: "button", title: target, onclick: () => browse(target) }, target));
    }
  }
  roots.append(el("h3", {}, "Places"));
  for (const root of result.roots) {
    roots.append(el("button", { type: "button", title: root.path, onclick: () => browse(root.path) }, root.name));
  }

  const folders = $("picker-folders");
  folders.replaceChildren();
  if (!pickerPath) {
    folders.append(el("li", { class: "picker-empty" }, "Pick a drive or place on the left, or type a path above."));
    return;
  }
  if (!result.error && result.folders.length === 0) {
    folders.append(el("li", { class: "picker-empty" }, "No subfolders. Use this folder, or go up."));
  }
  for (const folder of result.folders) {
    folders.append(el("li", {}, el("button", { type: "button", title: folder.path, onclick: () => browse(folder.path) }, folder.name)));
  }
}

function showPickerError(message) {
  $("picker-error").hidden = !message;
  $("picker-error").textContent = message || "";
}

async function useFolder() {
  const path = $("picker-input").value.trim();
  try {
    await api("POST", "/api/target", { path, profile: $("picker-profile").value });
  } catch (err) {
    showPickerError(err.message);
    return;
  }
  $("picker").close();
  catalog = null;
  statusFilter = null;
  autoScanned = true;
  await start("scan");
}

// ---- Helpers ---------------------------------------------------------------

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
  $("choose").addEventListener("click", openPicker);
  $("scan").addEventListener("click", () => start("scan"));
  $("check").addEventListener("click", () => start("check"));
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
  $("search").addEventListener("input", renderRows);
  $("more-search").addEventListener("input", renderCatalog);
  $("picker-go").addEventListener("click", () => browse($("picker-input").value.trim()));
  $("picker-input").addEventListener("keydown", (e) => {
    if (e.key === "Enter") { e.preventDefault(); browse($("picker-input").value.trim()); }
  });
  $("picker-use").addEventListener("click", useFolder);
  refresh();
  // Keep "checked 5 min ago" fresh.
  setInterval(() => { if (state && !state.run) render(); }, 60000);
});

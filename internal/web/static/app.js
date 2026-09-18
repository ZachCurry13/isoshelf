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

// What the list shows, kept in the browser between visits.
const view = {
  category: "",
  arch: "",
  sort: "attention",
  desc: false,
  updatesOnly: false,
  favoritesOnly: false,
  olderOnly: false,
};

function loadView() {
  try {
    Object.assign(view, JSON.parse(localStorage.getItem("isoshelf.view") || "{}"));
  } catch {
    // A browser that will not remember settings is fine; the defaults apply.
  }
  // "Recently changed" became "Recently added": a copied file keeps its old
  // change date, so it never answered the question people were asking.
  if (view.sort === "modified") view.sort = "added";
  $("category").value = view.category;
  $("arch").value = view.arch;
  $("sort").value = view.sort;
  $("only-updates").checked = view.updatesOnly;
  $("only-favorites").checked = view.favoritesOnly;
  $("only-older").checked = view.olderOnly;
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
  // A mistake while drawing must never stop the page from updating.
  try {
    render();
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
  $("space").textContent = state.space && state.space.total
    ? `${formatBytes(state.space.free)} free of ${formatBytes(state.space.total)}`
    : "";

  renderRun();

  if (state.error) showNotice(state.error, true);
  else if (state.warnings.length) showNotice(state.warnings.join(" "), false);
  else $("notice").hidden = true;

  $("changed-banner").hidden = !state.folder_changed;
  $("changed-scan").disabled = busy;

  renderSummary();
  renderHeadings();
  renderRows();
  renderFooter();
  renderCatalog();
  renderPast();
}

function renderRun() {
  const run = state.run;
  $("run").hidden = !run;
  if (!run) return;
  let text = "Scanning the folder…";
  let fraction = null;
  if (run.stage === "downloading") {
    text = run.file ? `Downloading ${run.file}…` : "Downloading…";
    if (run.total > 0) {
      text += ` ${formatBytes(run.done)} of ${formatBytes(run.total)}`;
      fraction = run.done / run.total;
    }
  } else if (run.stage === "verifying") {
    text = "Checking the downloaded file…";
    if (run.total > 0) fraction = run.done / run.total;
  } else if (run.stage === "placing") {
    text = "Putting the file in place…";
  } else if (run.stage === "hashing") {
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

function clearFilters() {
  statusFilter = null;
  view.category = view.arch = "";
  view.updatesOnly = view.favoritesOnly = view.olderOnly = false;
  $("search").value = "";
  $("category").value = $("arch").value = "";
  $("only-updates").checked = $("only-favorites").checked = $("only-older").checked = false;
  saveView();
  renderSummary();
  renderRows();
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
  const items = sortItems(state.report.items.filter((item) => {
    const haystack = `${item.name} ${item.path || ""} ${item.entry || ""} ${item.family || ""}`.toLowerCase();
    const starred = item.entry && (state.tracks[item.entry] || {}).starred;
    return (!statusFilter || item.status === statusFilter) &&
      (!query || haystack.includes(query)) &&
      (!view.category || item.category === view.category) &&
      (!view.arch || item.arch === view.arch) &&
      (!view.updatesOnly || item.status === "update available") &&
      (!view.favoritesOnly || starred) &&
      (!view.olderOnly || item.older);
  }));

  for (const item of items) rows.append(renderRow(item));
  // Like the key on a menu: only there when something in the list has the mark.
  $("legend").hidden = !items.some((item) => cautionOf(item));

  const total = state.report.items.length;
  $("shown").textContent = items.length === total ? plural(total, "image") : `${items.length} of ${plural(total, "image")}`;

  // A folder collects older copies: one downloaded by hand, one isoshelf
  // fetched, one from last year. Offer to clear them in one go.
  const older = state.report.items.filter((it) => it.older && it.path);
  const tidy = $("older-banner");
  tidy.hidden = older.length === 0;
  if (older.length) {
    const bytes = older.reduce((sum, it) => sum + (it.size || 0), 0);
    $("older-text").textContent =
      `${plural(older.length, "older copy", "older copies")} of images you already have${bytes ? `, using ${formatBytes(bytes)}` : ""}.`;
    $("older-clear").disabled = Boolean(state.run);
    $("older-clear").onclick = () => clearOlder(older);
    $("older-show").onclick = () => {
      statusFilter = null;
      view.olderOnly = true;
      $("only-older").checked = true;
      saveView();
      renderSummary();
      renderRows();
    };
  }

  // Updates get their own line above the list rather than a button at the
  // end of the filters, where it was easy to miss.
  const updatable = state.report.items.filter((it) => it.entry && it.updates === "download" && it.status === "update available");
  const banner = $("updates-banner");
  banner.hidden = updatable.length === 0;
  if (updatable.length) {
    const bytes = updatable.reduce((sum, it) => sum + (it.size || 0), 0);
    $("updates-text").textContent = updatable.length === 1
      ? `${updatable[0].name} has an update.`
      : `${plural(updatable.length, "image")} have updates${bytes ? `, replacing about ${formatBytes(bytes)}` : ""}.`;
  }
  const all = $("update-all");
  all.disabled = Boolean(state.run);
  all.textContent = updatable.length === 1 ? "Update it" : `Update all ${updatable.length}`;
  empty.hidden = items.length > 0;
  empty.replaceChildren();
  if (items.length === 0) {
    if (total === 0) {
      empty.append("No images found in this folder.");
    } else {
      const active = [
        statusFilter && `status "${statusFilter}"`,
        view.category && $("category").selectedOptions[0].textContent,
        view.arch && $("arch").selectedOptions[0].textContent,
        view.updatesOnly && "updates only",
        view.favoritesOnly && "favourites only",
        view.olderOnly && "older copies only",
        $("search").value.trim() && `search "${$("search").value.trim()}"`,
      ].filter(Boolean);
      empty.append(
        `None of the ${plural(total, "image")} here match ${active.join(", ")}.`,
        el("div", {},
          el("button", { type: "button", class: "btn small", onclick: clearFilters }, "Clear filters")));
    }
  }
}

const STATUS_ORDER = STATUSES.map(([status]) => status);

// compareVersions reads versions the way people do: 22.10 is newer than 22.4.
function compareVersions(a, b) {
  const partsOf = (v) => (v || "").split(/[^a-zA-Z0-9]+/).flatMap((part) => part.match(/\d+|[a-zA-Z]+/g) || []);
  const left = partsOf(a), right = partsOf(b);
  for (let i = 0; i < Math.max(left.length, right.length); i++) {
    const x = left[i], y = right[i];
    if (x === undefined) return -1;
    if (y === undefined) return 1;
    const bothNumbers = /^\d+$/.test(x) && /^\d+$/.test(y);
    if (bothNumbers && Number(x) !== Number(y)) return Number(x) - Number(y);
    if (!bothNumbers && x !== y) return x < y ? -1 : 1;
  }
  return 0;
}

// Each sorter puts the most useful end first, so "descending" means the
// reverse of what the column's name suggests: largest, newest, most urgent.
function sorters() {
  const byName = (a, b) => a.name.localeCompare(b.name) || (a.path || "").localeCompare(b.path || "");
  const starred = (item) => (item.entry && (state.tracks[item.entry] || {}).starred ? 0 : 1);
  const rank = (item) => STATUS_ORDER.indexOf(item.status);
  const text = (value) => (value || "").toLowerCase();
  // An image with nothing to replace sorts after both settings, because the
  // switch isn't shown for it at all.
  const replace = (item) => {
    if (!item.entry || item.updates !== "download") return 2;
    return (state.tracks[item.entry] || {}).keep_old ? 1 : 0;
  };
  return {
    attention: (a, b) => rank(a) - rank(b) || byName(a, b),
    favorites: (a, b) => starred(a) - starred(b) || rank(a) - rank(b) || byName(a, b),
    name: byName,
    size: (a, b) => (b.size || 0) - (a.size || 0) || byName(a, b),
    version: (a, b) => compareVersions(b.version, a.version) || byName(a, b),
    latest: (a, b) => compareVersions(b.latest, a.latest) || byName(a, b),
    file: (a, b) => text(a.path).localeCompare(text(b.path)) || byName(a, b),
    replace: (a, b) => replace(a) - replace(b) || byName(a, b),
    modified: (a, b) => new Date(b.modified || 0) - new Date(a.modified || 0) || byName(a, b),
    added: (a, b) => new Date(b.added || 0) - new Date(a.added || 0) || byName(a, b),
  };
}

// blank says whether a row has nothing to compare in this column: an image
// that isn't in the folder has no file, size or version. Those always sort
// last, whichever way round the column is, because a list that starts with
// blanks is a list you have to scroll past.
const BLANK = {
  size: (item) => !item.path || !item.size,
  version: (item) => !item.version,
  latest: (item) => !item.latest,
  file: (item) => !item.path,
  modified: (item) => !item.modified,
  added: (item) => !item.added,
};

function sortItems(items) {
  const all = sorters();
  const compare = all[view.sort] || all.attention;
  const isBlank = BLANK[view.sort] || (() => false);

  const filled = [], blanks = [];
  for (const item of items) (isBlank(item) ? blanks : filled).push(item);
  filled.sort(compare);
  if (view.desc) filled.reverse();
  blanks.sort(all.name);
  return [...filled, ...blanks];
}

// Columns that can be sorted by clicking their heading, and what each one
// means the first time it's clicked.
const COLUMN_SORTS = [
  ["col-status", "attention", "Most urgent first"],
  ["col-image", "name", "By name, A to Z"],
  ["col-version", "version", "Newest version here first"],
  ["col-latest", "latest", "Newest available first"],
  ["col-file", "file", "By filename, A to Z"],
  ["col-size", "size", "Largest first"],
  ["col-added", "added", "Most recently added first"],
  ["col-replace", "replace", "Images set to replace first"],
];

// renderHeadings makes the column titles sort the list, and shows which one
// is doing it.
function renderHeadings() {
  for (const [id, key, hint] of COLUMN_SORTS) {
    const heading = $(id);
    if (!heading) continue;
    const active = view.sort === key;
    heading.classList.toggle("sorted", active);
    heading.setAttribute("aria-sort", active ? (view.desc ? "descending" : "ascending") : "none");
    heading.title = active
      ? `Sorted ${view.desc ? "the other way round" : "this way"}. Click to reverse it.`
      : hint;
    heading.onclick = () => {
      // The same column again turns it round; a different one starts fresh.
      view.desc = active ? !view.desc : false;
      view.sort = key;
      $("sort").value = key;
      saveView();
      renderHeadings();
      renderRows();
    };
  }
}

// logoTile is the project logo, or coloured initials when there is none.
function logoTile(item) {
  const tile = el("span", { class: "logo", "aria-hidden": "true" });
  if (item.icon) {
    tile.classList.add("has-logo");
    tile.style.setProperty("--logo", `url("/logo/${encodeURIComponent(item.icon)}.svg")`);
    tile.style.setProperty("--brand", readableBrand(item.icon_color));
  } else {
    tile.classList.add("initials");
    tile.textContent = initials(item.name);
    tile.style.setProperty("--brand", colorFor(item.name));
  }
  return tile;
}

function initials(name) {
  const words = name.replace(/[^A-Za-z0-9 ]/g, " ").split(/\s+/).filter(Boolean);
  return ((words[0] || "?")[0] + (words[1] ? words[1][0] : "")).toUpperCase();
}

function colorFor(name) {
  let hash = 0;
  for (const ch of name) hash = (hash * 31 + ch.charCodeAt(0)) % 360;
  return `hsl(${hash} 45% 42%)`;
}

// readableBrand keeps brand colours visible: a few are nearly black, which
// disappears on a dark background.
function readableBrand(color) {
  if (!color) return "currentColor";
  const value = parseInt(color.slice(1), 16);
  const [r, g, b] = [(value >> 16) & 255, (value >> 8) & 255, value & 255];
  const luminance = (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255;
  const dark = window.matchMedia("(prefers-color-scheme: dark)").matches;
  if (dark && luminance < 0.25) return "#c9d2df";
  if (!dark && luminance > 0.9) return "#5f6878";
  return color;
}

// problemLink reports an image whose source has moved or broken. Projects
// rearrange their download folders without warning, and the person who hits
// it is the one who can say what happened.
function problemLink(item) {
  if (!state.report_url || !item.entry) return null;
  const title = `Catalog: ${item.name} ${item.status === "check failed" ? "can't be checked" : "needs fixing"}`;
  const body = [
    `**Image:** ${item.name} (\`${item.entry}\`)`,
    `**isoshelf:** ${state.version}`,
    item.version ? `**Version here:** ${item.version}` : null,
    item.latest ? `**Newest isoshelf found:** ${item.latest}` : null,
    `**Status:** ${item.status}`,
    item.note ? `**What it said:** ${item.note}` : null,
    "",
    "What's wrong? For a moved download, the new address helps most — especially",
    "where the project publishes its checksums now.",
  ].filter((line) => line !== null).join("\n");
  return ["Report a problem", `${state.report_url}?title=${encodeURIComponent(title)}&body=${encodeURIComponent(body)}&labels=catalog`];
}

// linksMenu holds the project pages, out of the way until asked for.
function linksMenu(item) {
  const links = [
    ["Download page", item.page],
    ["Website", item.site],
    ["Forum", item.forum],
    ["Release notes", item.release],
    problemLink(item),
  ].filter((link) => link && link[1]);
  if (!links.length) return null;

  const menu = el("details", { class: "menu" },
    el("summary", { title: "Links", "aria-label": `Links for ${item.name}` }, "\u2026"),
    el("div", { class: "menu-items" },
      links.map(([label, url]) => el("a", { href: url, target: "_blank", rel: "noopener noreferrer" }, label))));
  menu.addEventListener("toggle", () => {
    if (!menu.open) return;
    for (const other of document.querySelectorAll("details.menu[open]")) {
      if (other !== menu) other.open = false;
    }
  });
  return menu;
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
  const caution = cautionOf(item);
  const imageCell = el("td", {}, el("div", { class: "image-cell" },
    logoTile(item),
    el("div", { class: "image-text" },
      el("div", {},
        el("span", { class: "name" }, name),
        caution ? el("span", { class: "caution", title: caution, "aria-label": `Worth knowing: ${caution}` }, "⚠") : null,
        item.arch ? el("span", { class: "arch" }, item.arch) : null),
      item.note ? el("div", { class: "note" }, item.note) : null)));

  const newer = item.latest && item.status === "update available";
  const latestCell = el("td", { class: "latest-cell" },
    item.latest ? el("span", { class: newer ? "latest-new" : "", title: item.latest_file || "" }, item.latest) : "–");

  const fileCell = el("td", {},
    item.path
      ? [
        el("div", { class: "file" }, item.path),
        item.kind && item.kind !== "unknown" ? el("div", { class: "size" }, item.kind) : null,
      ]
      : el("span", { class: "muted" }, "Not in this folder"));

  // Size gets a column of its own, so the shelf can be sorted by what's
  // taking up the room.
  const sizeCell = el("td", { class: "size-cell" },
    item.path && item.size ? formatBytes(item.size) : el("span", { class: "muted" }, "–"));

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

  const actions = [];
  if (item.entry && item.updates === "download" && item.status === "update available") {
    actions.push(el("button", {
      type: "button", class: "btn small primary", disabled: busy,
      title: `Download ${item.latest || "the newest version"} and put it in this folder`,
      onclick: () => updateItem(item),
    }, "Update"));
  }
  if (item.path && !item.entry) {
    actions.push(el("button", {
      type: "button", class: "btn small primary", disabled: busy,
      title: "Let isoshelf work out what this file is",
      "aria-label": `Identify ${item.path}`,
      onclick: () => openIdentify(item),
    }, "What is this?"));
  }
  if (item.path && item.assigned) {
    actions.push(el("button", {
      type: "button", class: "btn small", disabled: busy,
      title: "You told isoshelf what this file is. Change that.",
      "aria-label": `Change what ${item.path} is`,
      onclick: () => openIdentify(item),
    }, "Not right?"));
  }
  if (item.path) {
    actions.push(el("button", {
      type: "button", class: "btn small", disabled: busy,
      title: "Remove this file from the folder",
      "aria-label": `Remove ${item.path}`,
      onclick: () => removeItem(item),
    }, "Remove"));
  }

  const menu = linksMenu(item);
  if (menu) actions.push(menu);

  return el("tr", {},
    el("td", { class: "col-star" }, star),
    statusCell,
    imageCell,
    el("td", { class: "version-cell" }, item.version || "–"),
    latestCell,
    fileCell,
    sizeCell,
    el("td", { class: "added-cell", title: item.added ? new Date(item.added).toLocaleString() : "" },
      item.added ? shortDate(item.added) : el("span", { class: "muted" }, "–")),
    el("td", { class: "replace" }, replace),
    el("td", { class: "row-actions" }, actions));
}

// cautionOf says what's worth knowing before using an image, if anything:
// that it no longer gets security fixes, or a note from the catalog. It's a
// mark to hover over, never a block — people keep old images on purpose.
function cautionOf(item) {
  if (item.caution) return item.caution;
  if (item.eol) return "End of life: this release no longer gets security fixes.";
  return "";
}

// shortDate reads like a person: "today", "3 days ago", then "12 Mar 2025".
function shortDate(iso) {
  const then = new Date(iso);
  const days = Math.floor((Date.now() - then.getTime()) / 86400000);
  if (days < 1) return "today";
  if (days === 1) return "yesterday";
  if (days < 7) return `${days} days ago`;
  return then.toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric" });
}

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

// removalChoice asks what should happen to the files an update replaces.
async function removalChoice(item) {
  const track = (item.entry && state.tracks[item.entry]) || {};
  if (!item.path) return "keep";
  // Images whose filename never changes land on top of the old file, so
  // keeping both isn't possible, whatever the switch says.
  const sameName = item.latest_file && item.path.split("/").pop() === item.latest_file;
  if (track.keep_old && !sameName) return "keep";

  // Moving aside keeps the old file on the drive, which is the safe answer
  // until the drive is nearly full: then keeping both is what makes the next
  // download fail.
  const room = item.size ? formatBytes(item.size) : null;
  const tight = Boolean(item.size && state.space && state.space.total &&
    state.space.free - item.size < Math.min(state.space.total / 100, 1 << 30));
  const preferReplace = tight || state.replace_action === "delete";

  const replace = {
    label: room ? `Replace it (frees ${room})` : "Replace it",
    value: "delete",
    primary: preferReplace,
  };
  const aside = {
    label: room ? `Archive it (still uses ${room}, undo any time)` : "Archive it",
    value: "move-aside",
    primary: !preferReplace,
  };
  const choices = preferReplace ? [replace, aside] : [aside, replace];
  if (!sameName) choices.push({ label: "Keep both where they are", value: "keep" });
  choices.push({ label: "Cancel", value: null });

  const what = sameName
    ? `The new file has the same name, so it takes the place of ${item.path}. It is downloaded and checked first.`
    : `The new file is downloaded and checked first, then ${item.path} is dealt with.`;
  const space = tight
    ? " There isn't room for both, so archiving the old one would leave the next download short."
    : " Archiving keeps it in this folder, under “Images that were here”, until you empty it.";

  const answer = await ask(`Update ${item.name}`, what + space, choices);
  // Remember which way they went, so the same question comes pre-answered.
  if (answer === "delete" || answer === "move-aside") {
    rememberReplaceAction(answer);
  }
  return answer;
}

async function updateItem(item) {
  const removal = await removalChoice(item);
  if (!removal) return;
  await startUpdate(item.entry, removal);
}

async function startUpdate(entry, removal) {
  try {
    await api("POST", "/api/update", { entry, removal });
    catalog = null;
  } catch (err) {
    showNotice(err.message, true);
  }
  await refresh();
}

async function updateAll() {
  const items = state.report.items.filter((it) => it.entry && it.updates === "download" && it.status === "update available");
  if (!items.length) return;
  const removal = await ask(
    `Update ${plural(items.length, "image")}`,
    "Each one is downloaded and checked before anything is replaced. What should happen to the old files? Images set to keep old files are left alone.",
    [
      { label: "Move them aside", value: "move-aside", primary: true },
      { label: "Delete them", value: "delete" },
      { label: "Keep them", value: "keep" },
      { label: "Cancel", value: null },
    ]);
  if (!removal) return;

  for (const item of items) {
    const track = state.tracks[item.entry] || {};
    await startUpdate(item.entry, track.keep_old ? "keep" : removal);
    // Wait for this download to finish before starting the next one.
    while (state.run) {
      await new Promise((done) => setTimeout(done, 600));
      await refresh();
    }
    if (state.error) return;
  }
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
    render();
  } catch (err) {
    showNotice(err.message, true);
  }
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
    el("span", {}, `${plural(cat.entries, "image")} known: ${where}${when}${mine}.`));

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

  const shown = filterCatalog(missing);
  const list = $("catalog");
  list.replaceChildren();
  for (const entry of shown) {
    const room = fitsHere(entry);
    list.append(el("li", {},
      logoTile(entry),
      el("div", { class: "info" },
        el("div", { class: "info-line" },
          el("span", { class: "name" }, entry.name),
          el("span", { class: "arch" }, entry.arch),
          entry.popular ? el("span", { class: "pill s-ok", title: "Turns up in public round-ups of what people are running. A hand-picked hint, not a rating." }, "popular") : null),
        el("div", { class: "kind" },
          entry.size ? `about ${formatBytes(entry.size)} · ` : "",
          UPDATES_LABEL[entry.updates] || entry.updates,
          room === false ? el("span", { class: "wont-fit" }, " · bigger than the room left here") : null)),
      entry.page ? el("a", { class: "btn small", href: entry.page, target: "_blank", rel: "noopener noreferrer" }, "Page") : null,
      addButton(entry, room)));
  }
  $("more-shown").textContent = shown.length === missing.length
    ? ""
    : `showing ${shown.length} of ${missing.length}`;

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

// fitsHere reports whether an image would fit in the folder: true, false, or
// null when either the size or the free space is unknown.
function fitsHere(entry) {
  if (!entry.size || !state.space || !state.space.total) return null;
  const spare = Math.min(state.space.total / 100, 1 << 30);
  return state.space.free - entry.size >= spare;
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

  const shown = entries.filter((entry) => {
    if (query && !`${entry.name} ${entry.id} ${entry.family || ""}`.toLowerCase().includes(query)) return false;
    if (category && (entry.category || "other") !== category) return false;
    if (arch && entry.arch !== arch) return false;
    if (updates && entry.updates !== updates) return false;
    if (fitsOnly && fitsHere(entry) === false) return false;
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
    return el("button", {
      type: "button", class: "btn small", disabled: true,
      title: `${entry.name} is about ${formatBytes(entry.size)}, and this folder has ${formatBytes(state.space.free)} left.`,
    }, "Add");
  }
  if (entry.updates !== "download") {
    return el("button", {
      type: "button", class: "btn small", disabled: true,
      title: entry.page
        ? `isoshelf can't download ${entry.name} itself, because there is nowhere to check it against. Use its download page.`
        : `isoshelf can't download ${entry.name} itself: there is nowhere to check it against.`,
    }, "Add");
  }
  return el("button", {
    type: "button", class: "btn small primary", disabled: Boolean(state.run),
    title: `Download the newest ${entry.name} into this folder`,
    onclick: () => startUpdate(entry.id, "keep"),
  }, "Add");
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

async function renderPast() {
  if (!state.target) {
    $("past-count").textContent = "";
    $("past-list").replaceChildren();
    return;
  }
  let items;
  try {
    items = (await api("GET", "/api/archive")).items;
  } catch {
    return;
  }
  $("past").hidden = items.length === 0;
  $("past-count").textContent = plural(items.length, "image");

  const list = $("past-list");
  list.replaceChildren();
  for (const item of items) {
    const when = item.gone_at ? timeAgo(item.gone_at) : "";
    const detail = [GONE_LABEL[item.gone] || item.gone, when, formatBytes(item.size)].filter(Boolean).join(" \u00b7 ");
    const buttons = [];
    if (item.restorable) {
      buttons.push(el("button", {
        type: "button", class: "btn small", disabled: Boolean(state.run),
        title: "Move it back into the folder",
        onclick: () => restore(item),
      }, "Put back"));
    }
    if (item.downloadable) {
      buttons.push(el("button", {
        type: "button", class: "btn small", disabled: Boolean(state.run),
        title: "Download the current version again",
        onclick: () => startUpdate(item.entry, "keep"),
      }, "Download again"));
    }
    if (item.page) {
      buttons.push(el("a", { class: "btn small", href: item.page, target: "_blank", rel: "noopener noreferrer" }, "Page"));
    }
    list.append(el("li", {},
      logoTile(item),
      el("div", { class: "info" },
        el("div", {}, el("span", { class: "name" }, item.name),
          item.version ? el("span", { class: "arch" }, item.version) : null),
        el("div", { class: "kind" }, `${item.path} \u00b7 ${detail}`)),
      buttons));
  }
}

async function restore(item) {
  try {
    await api("POST", "/api/restore", { name: item.path.split("/").pop() });
  } catch (err) {
    showNotice(err.message, true);
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

  // How many images a folder holds says more than its path does, and on
  // Windows a drive's name is the only way to tell E: from F:.
  const describe = (folder) => {
    const bits = [folder.label, folder.images > 0 ? plural(folder.images, "image") : null].filter(Boolean);
    return bits.length ? el("span", { class: "muted" }, ` — ${bits.join(", ")}`) : null;
  };
  $("picker-here").textContent = result.images > 0
    ? `${plural(result.images, "image")} in this folder`
    : result.path && result.images === 0 ? "No images directly in this folder" : "";

  const roots = $("picker-roots");
  roots.replaceChildren();
  const bookmarks = (state && state.bookmarks) || [];
  if (bookmarks.length) {
    roots.append(el("h3", {}, "Bookmarks"));
    for (const target of bookmarks) {
      roots.append(el("button", { type: "button", title: target, onclick: () => browse(target) }, target));
    }
  }
  if (state && state.recent_targets.length) {
    roots.append(el("h3", {}, "Recent"));
    for (const target of state.recent_targets) {
      if (bookmarks.includes(target)) continue;
      roots.append(el("button", { type: "button", title: target, onclick: () => browse(target) }, target));
    }
  }
  roots.append(el("h3", {}, "Places"));
  for (const root of result.roots) {
    roots.append(el("button", { type: "button", title: root.path, onclick: () => browse(root.path) },
      root.name, describe(root)));
  }

  // The pin follows whichever folder is open.
  const pinned = bookmarks.some((b) => b.toLowerCase() === pickerPath.toLowerCase());
  const pin = $("picker-pin");
  pin.disabled = !pickerPath;
  pin.textContent = pinned ? "★ Bookmarked" : "☆ Bookmark";
  pin.title = pinned
    ? "Forget this folder"
    : "Keep this folder at the top of the list, so it doesn't have to be found again";
  pin.onclick = () => toggleBookmark(pickerPath, pinned);

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
  $("update-all").addEventListener("click", updateAll);
  $("changed-scan").addEventListener("click", () => start("scan"));
  $("search").addEventListener("input", renderRows);
  loadView();
  for (const [id, key] of [["category", "category"], ["arch", "arch"]]) {
    $(id).addEventListener("change", (e) => { view[key] = e.target.value; saveView(); renderRows(); });
  }
  // Picking a sort from the list means the way that option is worded:
  // "largest first" is already the right way round.
  $("sort").addEventListener("change", (e) => {
    view.sort = e.target.value;
    view.desc = false;
    saveView();
    renderHeadings();
    renderRows();
  });
  for (const [id, key] of [["only-updates", "updatesOnly"], ["only-favorites", "favoritesOnly"], ["only-older", "olderOnly"]]) {
    $(id).addEventListener("change", (e) => { view[key] = e.target.checked; saveView(); renderRows(); });
  }
  $("more-search").addEventListener("input", renderCatalog);
  for (const id of ["more-category", "more-arch", "more-sort", "more-updates", "more-fits"]) {
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

// toggleBookmark pins or unpins a folder in the chooser.
async function toggleBookmark(path, pinned) {
  try {
    state = await api("POST", "/api/bookmark", { path, remove: pinned });
  } catch (err) {
    showPickerError(err.message);
    return;
  }
  await browse(pickerPath);
}

// rememberReplaceAction stores which answer the user gives when an update
// replaces a file, so they aren't asked the same thing from scratch forever.
// They are still asked: it only decides which button is the ready one.
async function rememberReplaceAction(action) {
  if (state.replace_action === action) return;
  try {
    state = await api("POST", "/api/settings", { replace_action: action });
  } catch {
    // Not remembering is a small thing; the update itself carries on.
  }
}

// clearOlder gets rid of every older copy at once, after showing exactly
// which files it means. The same two ways out as any other removal: archive
// them, or delete them now.
async function clearOlder(older) {
  const bytes = older.reduce((sum, it) => sum + (it.size || 0), 0);
  const names = older.map((it) => it.path);
  const listed = names.length > 6
    ? `${names.slice(0, 6).join("\n")}\nand ${names.length - 6} more`
    : names.join("\n");

  const how = await ask(
    `Clear ${plural(older.length, "older copy", "older copies")}?`,
    `These are images you have a newer copy of, using ${formatBytes(bytes)}:\n\n${listed}\n\n` +
    "Archiving keeps them in this folder until you empty it; deleting frees the space now.",
    [
      { label: `Delete them (frees ${formatBytes(bytes)})`, value: "delete" },
      { label: "Archive them", value: "move-aside", primary: true },
      { label: "Cancel", value: null },
    ]);
  if (!how) return;

  try {
    state = await api("POST", "/api/remove", { paths: names, how });
    catalog = null;
  } catch (err) {
    showNotice(err.message, true);
    return;
  }
  showNotice(`${plural(names.length, "older copy", "older copies")} ${how === "delete" ? "deleted" : "archived"}.`);
  await start("scan");
}

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

// ---- Downloads -------------------------------------------------------------

// Downloads run one at a time, in the queue's order, which can be changed —
// like a game launcher's download queue. The bar at the bottom of the page
// shows the one running; opening it shows the rest.

let dockOpen = false;
// dockDrawn is what the list last showed, so it is only rebuilt when that
// changes; dragging is the id of the download being dragged, if one is.
let dockDrawn = "";
let dragging = null;
// dockCurrent holds the running download's line and bar in the open list.
let dockCurrent = null;
// speed smooths the download rate over the last few seconds.
let speed = { id: null, done: 0, at: 0, rate: 0 };

function downloads() {
  return (state && state.downloads) || { queued: [], finished: [] };
}

// jobFor says where an image is in the downloads: downloading now, waiting
// (and at which place), or how it ended. Null when it isn't there.
function jobFor(entry) {
  const d = downloads();
  if (d.current && d.current.entry === entry) return { where: "current", job: d.current };
  const at = d.queued.findIndex((job) => job.entry === entry);
  if (at >= 0) return { where: "queued", job: d.queued[at], place: at + 1 };
  const ended = d.finished.find((job) => job.entry === entry);
  return ended ? { where: ended.outcome, job: ended } : null;
}

// inDownloads is true while an image is downloading or waiting, or has just
// arrived and the folder hasn't been looked at since.
function inDownloads(entry) {
  const at = jobFor(entry);
  if (!at) return false;
  return at.where === "current" || at.where === "queued" || (at.where === "done" && arrivedSinceScan(at.job));
}

function arrivedSinceScan(job) {
  return !state.updated_at || new Date(job.at) > new Date(state.updated_at);
}

// pendingBytes is roughly how much the queue still has to write.
function pendingBytes() {
  const d = downloads();
  let bytes = d.queued.reduce((sum, job) => sum + (job.size || 0), 0);
  if (d.current) {
    const run = state.run;
    bytes += run && run.total ? Math.max(run.total - run.done, 0) : d.current.size || 0;
  }
  return bytes;
}

// downloadProgress describes the download that is running, in words and as a
// fraction (null when there's no telling).
function downloadProgress() {
  const run = state.run;
  if (!run || run.kind !== "update") return { text: "Starting…", fraction: null, short: "Starting…" };
  if (run.stage === "downloading" && run.total > 0) {
    const fraction = run.done / run.total;
    const pct = `${Math.floor(fraction * 100)}%`;
    let text = `${formatBytes(run.done)} of ${formatBytes(run.total)}`;
    const rate = downloadRate(run);
    if (rate > 0) {
      text += ` · ${formatBytes(rate)}/s`;
      const left = (run.total - run.done) / rate;
      if (left > 0 && left < 86400) text += ` · ${duration(left)} left`;
    }
    return { text, fraction, short: `Downloading ${pct}` };
  }
  if (run.stage === "downloading") return { text: "Downloading…", fraction: null, short: "Downloading…" };
  if (run.stage === "verifying") {
    return {
      text: "Checking it against the published checksum…",
      fraction: run.total > 0 ? run.done / run.total : null,
      short: "Checking…",
    };
  }
  if (run.stage === "placing") return { text: "Putting it in place…", fraction: 1, short: "Almost done" };
  return { text: "Finding the download…", fraction: null, short: "Starting…" };
}

function downloadRate(run) {
  const d = downloads();
  const id = d.current && d.current.id;
  const now = Date.now();
  if (speed.id !== id || run.done < speed.done) {
    speed = { id, done: run.done, at: now, rate: 0 };
    return 0;
  }
  const seconds = (now - speed.at) / 1000;
  if (seconds >= 1) {
    const rate = (run.done - speed.done) / seconds;
    speed = { id, done: run.done, at: now, rate: speed.rate ? speed.rate * 0.7 + rate * 0.3 : rate };
  }
  return speed.rate;
}

function duration(seconds) {
  if (seconds < 90) return "under 2 min";
  const minutes = Math.round(seconds / 60);
  if (minutes < 90) return `about ${minutes} min`;
  return `about ${Math.round(minutes / 60)} h`;
}

function renderDock() {
  const d = downloads();
  const any = Boolean(d.current || d.queued.length || d.finished.length);
  $("dock").hidden = !any;
  document.body.classList.toggle("has-dock", any);
  if (!any) {
    dockOpen = false;
    return;
  }
  $("dock-panel").hidden = !dockOpen;
  $("dock-toggle").textContent = dockOpen ? "Hide" : "Show";
  $("dock-toggle").setAttribute("aria-expanded", String(dockOpen));
  $("dock-stop").hidden = !(d.current || d.queued.length);
  $("dock-clear").hidden = !d.finished.length;
  renderDockProgress();

  // Never rebuild the list under a drag, or while it hasn't changed: the
  // button that has focus would go with it.
  const key = JSON.stringify([dockOpen, d.current && d.current.id,
    d.queued.map((job) => job.id), d.finished.map((job) => [job.id, job.outcome])]);
  if (dragging !== null || key === dockDrawn) return;
  dockDrawn = key;

  const list = $("dock-list");
  const focused = rememberFocus();
  list.replaceChildren();
  dockCurrent = null;
  if (!dockOpen) return;
  if (d.current) {
    dockCurrent = {
      text: el("div", { class: "kind" }, downloadProgress().text),
      fill: el("div", { class: "bar-fill" }),
    };
    list.append(el("li", { class: "dock-heading" }, "Downloading"));
    list.append(el("li", { class: "dock-item current" },
      logoTile(entryInfo(d.current)),
      el("div", { class: "info" },
        el("div", { class: "name" }, d.current.name),
        dockCurrent.text,
        el("div", { class: "bar" }, dockCurrent.fill)),
      el("button", {
        type: "button", class: "btn small",
        title: "Stop this one and go on to the next. The part-finished download is kept.",
        "aria-label": `Stop downloading ${d.current.name}`,
        onclick: () => dropJob(d.current),
      }, "Stop")));
  }
  if (d.queued.length) {
    list.append(el("li", { class: "dock-heading" },
      `Up next · ${plural(d.queued.length, "image")}`,
      el("span", { class: "muted" }, " — drag to change the order")));
    d.queued.forEach((job, i) => list.append(queuedItem(job, i, d.queued.length)));
  }
  if (d.finished.length) {
    list.append(el("li", { class: "dock-heading" }, "Finished"));
    for (const job of d.finished) list.append(finishedItem(job));
  }
  renderDockProgress();
  restoreFocus(focused);
}

// rememberFocus and restoreFocus keep the keyboard on the download that was
// just moved, across the list being rebuilt; after one is taken off the
// queue, on its neighbour.
function rememberFocus() {
  const active = document.activeElement;
  if (!active || !active.dataset || !active.dataset.focus || !$("dock-list").contains(active)) return null;
  const [id, act] = active.dataset.focus.split(":");
  const index = [...$("dock-list").querySelectorAll("li.queued")].indexOf(active.closest("li"));
  return { id, act, index };
}

function restoreFocus(saved) {
  if (!saved) return;
  const list = $("dock-list");
  const same = list.querySelector(`[data-focus="${saved.id}:${saved.act}"]:not(:disabled)`) ||
    list.querySelector(`[data-focus^="${saved.id}:"]:not(:disabled)`);
  if (same) {
    same.focus();
    return;
  }
  const queued = [...list.querySelectorAll("li.queued")];
  const near = saved.index >= 0 ? queued[Math.min(saved.index, queued.length - 1)] : null;
  const button = near && (near.querySelector(`[data-focus$=":${saved.act}"]:not(:disabled)`) || near.querySelector("[data-focus]:not(:disabled)"));
  (button || $("dock-toggle")).focus();
}

// entryInfo finds what the page knows about an image, for its logo.
function entryInfo(job) {
  const known = (catalog || []).find((entry) => entry.id === job.entry) ||
    ((state.report && state.report.items) || []).find((item) => item.entry === job.entry);
  return known || { name: job.name };
}

function queuedItem(job, i, count) {
  const what = job.update ? "replaces the copy here" : "adds it to this folder";
  const li = el("li", { class: "dock-item queued", draggable: "true", "data-id": job.id },
    el("span", { class: "grip", "aria-hidden": "true", title: "Drag to change the order" }, "⠇"),
    el("span", { class: "place" }, i + 1),
    logoTile(entryInfo(job)),
    el("div", { class: "info" },
      el("div", { class: "name" }, job.name),
      el("div", { class: "kind" }, [job.size ? `about ${formatBytes(job.size)}` : null, what].filter(Boolean).join(" · "))),
    el("div", { class: "dock-buttons" },
      el("button", {
        type: "button", class: "btn small icon", disabled: i === 0,
        title: "Move up", "aria-label": `Move ${job.name} up`, "data-focus": `${job.id}:up`,
        onclick: () => moveJob(job, i - 1),
      }, "↑"),
      el("button", {
        type: "button", class: "btn small icon", disabled: i === count - 1,
        title: "Move down", "aria-label": `Move ${job.name} down`, "data-focus": `${job.id}:down`,
        onclick: () => moveJob(job, i + 1),
      }, "↓"),
      el("button", {
        type: "button", class: "btn small icon", disabled: i === 0,
        title: "Next in line", "aria-label": `Download ${job.name} next`, "data-focus": `${job.id}:top`,
        onclick: () => moveJob(job, 0),
      }, "⤒"),
      el("button", {
        type: "button", class: "btn small icon",
        title: "Take it off the queue", "aria-label": `Take ${job.name} off the queue`, "data-focus": `${job.id}:drop`,
        onclick: () => dropJob(job),
      }, "✕")));

  // Dragging: drop above or below another waiting download.
  li.addEventListener("dragstart", (e) => {
    dragging = job.id;
    e.dataTransfer.effectAllowed = "move";
    e.dataTransfer.setData("text/plain", String(job.id));
    li.classList.add("dragging");
  });
  li.addEventListener("dragend", () => {
    dragging = null;
    li.classList.remove("dragging");
    dockDrawn = "";
    renderDock();
  });
  li.addEventListener("dragover", (e) => {
    if (dragging === null || dragging === job.id) return;
    e.preventDefault();
    const rect = li.getBoundingClientRect();
    const below = e.clientY - rect.top > rect.height / 2;
    li.classList.toggle("drop-before", !below);
    li.classList.toggle("drop-after", below);
  });
  li.addEventListener("dragleave", () => li.classList.remove("drop-before", "drop-after"));
  li.addEventListener("drop", (e) => {
    e.preventDefault();
    const below = li.classList.contains("drop-after");
    li.classList.remove("drop-before", "drop-after");
    const queue = downloads().queued;
    const from = queue.findIndex((other) => other.id === dragging);
    if (from < 0) return;
    // The place counts the list without the one being moved.
    let to = i + (below ? 1 : 0);
    if (from < to) to -= 1;
    const moved = queue[from];
    dragging = null;
    if (to !== from) moveJob(moved, to);
  });
  return li;
}

const OUTCOME = {
  done: ["✓", "s-ok"],
  failed: ["✕", "s-bad"],
  stopped: ["■", "s-muted"],
};

function finishedItem(job) {
  let [mark, tone] = OUTCOME[job.outcome] || ["", "s-muted"];
  let what = job.update ? "Updated" : "Added";
  // Done, but with something to know: a file nothing could check.
  if (job.outcome === "done" && job.message) {
    [mark, tone] = ["!", "s-warn"];
    what += `. ${job.message}`;
  }
  if (job.outcome === "failed") what = `Didn't work: ${job.message}`;
  if (job.outcome === "stopped") what = "Stopped. The part-finished download is kept, so trying again carries on from there.";
  // Only the latest try gets the button, and not once it is queued again.
  const again = job.outcome !== "done" && jobFor(job.entry).job.id === job.id;
  return el("li", { class: "dock-item finished" },
    el("span", { class: `pill ${tone} mark`, "aria-hidden": "true" }, mark),
    el("div", { class: "info" },
      el("div", { class: "name" }, job.name),
      el("div", { class: "kind" }, what, job.at ? ` · ${timeAgo(job.at)}` : "")),
    again
      ? el("button", {
        type: "button", class: "btn small",
        title: "Add it to the queue again",
        onclick: () => retryJob(job),
      }, job.outcome === "stopped" ? "Carry on" : "Try again")
      : null);
}

// renderDockProgress updates the moving parts of the downloads: the summary
// on the bar, the running download's line, and the buttons that show it.
function renderDockProgress() {
  const d = downloads();
  if ($("dock").hidden) return;
  const running = d.current ? downloadProgress() : null;
  const count = (outcome) => d.finished.filter((job) => job.outcome === outcome).length;
  let text;
  if (d.current) {
    text = `${d.current.update ? "Updating" : "Adding"} ${d.current.name} — ${running.text}`;
  } else if (d.queued.length) {
    text = "Starting the next one…";
  } else {
    const parts = [
      count("done") && `${count("done")} done`,
      count("failed") && `${count("failed")} didn't work`,
      count("stopped") && `${count("stopped")} stopped`,
    ].filter(Boolean);
    text = `Finished: ${parts.join(", ")}.`;
    if (state.run) text += " Looking at the folder again…";
  }
  $("dock-text").textContent = text;
  $("dock-waiting").textContent = d.queued.length ? `${d.queued.length} waiting` : "";

  const bar = $("dock-bar");
  bar.hidden = !d.current;
  if (running) {
    bar.classList.toggle("indeterminate", running.fraction === null);
    $("dock-fill").style.width = running.fraction === null ? "" : `${Math.round(running.fraction * 100)}%`;
    if (dockCurrent) {
      dockCurrent.text.textContent = running.text;
      dockCurrent.fill.style.width = running.fraction === null ? "0" : `${Math.round(running.fraction * 100)}%`;
    }
  }
  for (const button of document.querySelectorAll("[data-downloading]")) {
    button.textContent = running ? running.short : "Starting…";
  }
}

function openDock() {
  dockOpen = true;
  renderDock();
  $("dock-toggle").focus();
}

// queueDownload adds an image to the downloads. It starts at once when
// nothing else is downloading.
async function queueDownload(entry, removal) {
  try {
    await api("POST", "/api/update", { entry, removal });
  } catch (err) {
    showNotice(err.message, true);
  }
  await refresh();
}

async function moveJob(job, position) {
  try {
    state.downloads = await api("POST", "/api/queue/move", { id: job.id, position });
  } catch (err) {
    showNotice(err.message, true);
  }
  await refresh();
}

async function dropJob(job) {
  try {
    state.downloads = await api("POST", "/api/queue/drop", { id: job.id });
  } catch (err) {
    showNotice(err.message, true);
  }
  await refresh();
}

// retryJob queues a download that failed or was stopped. An update asks
// again what to do with the old file, since that answer went with the job.
async function retryJob(job) {
  const item = job.update && state.report && state.report.items.find((it) => it.entry === job.entry && it.path);
  if (item) {
    await updateItem(item);
  } else {
    await queueDownload(job.entry, "keep");
  }
}

async function stopDownloads() {
  const d = downloads();
  const waiting = d.queued.length;
  const answer = await ask(
    "Stop all downloads?",
    `The download running now stops${waiting ? `, and ${plural(waiting, "image")} waiting ${waiting === 1 ? "is" : "are"} taken off the queue` : ""}. A part-finished download is kept, so adding the image again carries on from there.`,
    [{ label: "Stop them", value: "yes", primary: true }, { label: "Keep going", value: null }]);
  if (!answer) return;
  try {
    await api("POST", "/api/cancel");
  } catch (err) {
    showNotice(err.message, true);
  }
  await refresh();
}

async function clearFinished() {
  try {
    state.downloads = await api("POST", "/api/queue/clear");
  } catch (err) {
    showNotice(err.message, true);
  }
  await refresh();
}

// jobButton stands in for an Add or Update button while that image is in the
// downloads, so the button says what is happening to it. idle is the button
// to show otherwise; after a failure it offers to try again.
function jobButton(entry, idle, fresh) {
  const at = jobFor(entry);
  if (!at) return idle;
  switch (at.where) {
    case "current":
      return el("button", {
        type: "button", class: "btn small job-state", "data-downloading": "",
        title: "Downloading now. Click to see the downloads.",
        onclick: openDock,
      }, downloadProgress().short);
    case "queued":
      return el("button", {
        type: "button", class: "btn small job-state",
        title: `Waiting its turn: number ${at.place} in the queue. Click to change the order.`,
        onclick: openDock,
      }, `Queued #${at.place}`);
    case "done":
      // Once the folder has been looked at again, the list shows the file
      // itself and the tick has done its job.
      if (!fresh || arrivedSinceScan(at.job)) {
        return el("span", { class: "job-done" }, at.job.update ? "Updated ✓" : "Added ✓");
      }
      return idle;
    default:
      if (idle instanceof HTMLButtonElement && !idle.disabled) {
        idle.textContent = at.where === "stopped" ? "Carry on" : "Try again";
        idle.title = at.job.message || idle.title;
      }
      return idle;
  }
}

// flash is a short confirmation that outlives the redraws after it, which
// would otherwise hide it at once.
let flash = null;

function flashNotice(message) {
  flash = { message, until: Date.now() + 10000 };
  showNotice(message, false);
  setTimeout(() => { if (flash && Date.now() >= flash.until) { flash = null; render(); } }, 10100);
}

function showNotice(message, isError) {
  const notice = $("notice");
  notice.textContent = message;
  notice.classList.toggle("error", isError);
  notice.hidden = false;
}

// ---- Statuses in plain words ----------------------------------------------

// What the page calls each status, and what it means. The report keeps its
// own words for the command line and for scripts; only the page speaks
// plainly, and every status explains itself when you hover or tap it.
const STATUS_WORDS = {
  "update available": ["Update ready", "A newer version is published. isoshelf can download it, check it, and put it in place."],
  "up to date": ["Up to date", "This is the newest version the project publishes."],
  "EOL": ["Old release", "This release no longer gets security fixes. Fine to keep for a virtual machine, an old PC or tinkering."],
  "not bootable": ["Won't boot here", "An image, but not in a shape this folder's boot menu can use."],
  "manual": ["Check by hand", "The project publishes nothing isoshelf can check against, so updates are up to you. Its download page is one click away."],
  "missing": ["Missing", "You usually keep this image here, but the file isn't in the folder."],
  "unrecognized": ["Unknown file", "isoshelf doesn't know what this file is. It can work it out, or you can name it yourself."],
  "checksum mismatch": ["Doesn't match", "The file doesn't match the checksum the project publishes: a broken download, or a changed file."],
  "unverified": ["Not checked", "The project publishes no checksum for this file, so nothing can prove it arrived intact."],
  "check failed": ["Couldn't check", "isoshelf couldn't reach the project this time. The note says what went wrong."],
  "unknown": ["Not sure yet", "isoshelf hasn't worked out enough about this file to say."],
  "not checked": ["Not checked yet", "Press Check for updates and isoshelf will ask each project what's newest."],
};

function statusWord(status) {
  return (STATUS_WORDS[status] || [status])[0];
}

function statusHelp(status) {
  return (STATUS_WORDS[status] || [])[1] || "";
}

// showMeanings lists every status and what it means, for anyone who wants
// the whole key rather than one explanation at a time.
function showMeanings() {
  const dialog = $("ask");
  $("ask-title").textContent = "What the statuses mean";
  const text = $("ask-text");
  text.replaceChildren(el("dl", { class: "meanings" },
    Object.entries(STATUS_WORDS).flatMap(([status, [word, help]]) => [
      el("dt", {}, el("span", { class: `pill ${STATUS_CLASS[status] || "s-muted"}` }, word)),
      el("dd", {}, help),
    ])));
  const buttons = $("ask-buttons");
  buttons.replaceChildren(el("button", { type: "button", class: "btn primary", onclick: () => dialog.close() }, "Got it"));
  dialog.showModal();
}

// ---- Things to do ----------------------------------------------------------

// One card per thing that wants doing, instead of a stack of banners in
// different styles. A card only appears when there is something to do.
function renderTodo() {
  const todo = $("todo");
  todo.replaceChildren();
  if (!state.report) return;
  const items = state.report.items;
  const cards = [];

  const updates = updatable();
  const byHand = items.filter((it) => it.status === "update available" && it.updates !== "download").length;
  if (updates.length || byHand) {
    const waiting = updates.filter((it) => inDownloads(it.entry)).length;
    const left = updates.length - waiting;
    const bytes = updates.reduce((sum, it) => sum + (it.size || 0), 0);
    const all = updates.length + byHand;
    cards.push(todoCard({
      tone: "s-update",
      title: all === 1 ? "1 update ready" : `${all} updates ready`,
      detail: bytes ? `about ${formatBytes(bytes)} to download` : "",
      note: [
        byHand ? `${byHand} to download by hand` : "",
        waiting ? `${waiting} already in the downloads` : "",
      ].filter(Boolean).join(" 00b7 "),
      actions: [el("button", {
        type: "button", class: "btn primary", disabled: left === 0,
        onclick: updateAll,
      }, left === 0 ? (updates.length ? "All queued" : "By hand only")
        : left === all ? "Update all" : `Update ${left}`)],
    }));
  }

  const older = items.filter((it) => it.older && it.path);
  if (older.length) {
    const bytes = older.reduce((sum, it) => sum + (it.size || 0), 0);
    cards.push(todoCard({
      title: older.length === 1 ? "1 older version" : `${older.length} older versions`,
      detail: bytes ? `${formatBytes(bytes)} you could free` : "",
      note: "You have a newer version of each of these.",
      actions: [el("button", {
        type: "button", class: "btn", disabled: scanning(),
        onclick: () => reviewOlder(older),
      }, "Review…")],
    }));
  }

  if (state.removed && state.removed.files) {
    cards.push(todoCard({
      title: "Archive",
      detail: `${plural(state.removed.files, "file")} · ${formatBytes(state.removed.bytes)}`,
      note: "Set aside, still using room here.",
      actions: [
        el("button", { type: "button", class: "btn", onclick: () => jumpTo("archive") }, "View"),
        el("button", { type: "button", class: "btn", disabled: Boolean(state.run), onclick: emptyRemoved }, "Empty"),
      ],
    }));
  }

  const stuck = items.filter((it) => it.status === "not bootable");
  if (stuck.length) {
    cards.push(todoCard({
      tone: "s-warn",
      title: stuck.length === 1 ? "1 file won't boot here" : `${stuck.length} files won't boot here`,
      note: "An image, but not in a shape this folder's boot menu can use.",
      actions: [el("button", { type: "button", class: "btn", onclick: () => showOnly("not bootable") }, "Show them")],
    }));
  }

  const unknown = items.filter((it) => it.status === "unrecognized");
  if (unknown.length) {
    cards.push(todoCard({
      title: unknown.length === 1 ? "1 unknown file" : `${unknown.length} unknown files`,
      note: "isoshelf can work out what they are.",
      actions: [el("button", { type: "button", class: "btn", onclick: () => showOnly("unrecognized") }, "Show them")],
    }));
  }

  const missing = items.filter((it) => it.status === "missing");
  if (missing.length) {
    cards.push(todoCard({
      tone: "s-missing",
      title: missing.length === 1 ? "1 image missing" : `${missing.length} images missing`,
      note: "You usually keep these here.",
      actions: [el("button", { type: "button", class: "btn", onclick: () => showOnly("missing") }, "Show them")],
    }));
  }

  if (state.folder_changed) {
    cards.push(todoCard({
      title: "The folder changed",
      note: "Something was added or removed outside isoshelf.",
      actions: [el("button", {
        type: "button", class: "btn", disabled: Boolean(state.run),
        onclick: () => start("scan"),
      }, "Look again")],
    }));
  }

  todo.append(...cards);
}

function todoCard({ title, detail, note, actions, tone }) {
  return el("div", { class: `todo-card ${tone || ""}` },
    el("div", { class: "todo-title" }, title),
    detail ? el("div", { class: "todo-detail" }, detail) : null,
    note ? el("div", { class: "todo-note" }, note) : null,
    el("div", { class: "todo-actions" }, actions));
}

function jumpTo(id) {
  const section = $(id);
  const details = section.querySelector("details");
  if (details) details.open = true;
  section.scrollIntoView({ behavior: "smooth", block: "start" });
}

// showOnly filters the list down to one status, as a chip you can remove.
function showOnly(status) {
  view.status = status;
  saveView();
  renderFilters();
  renderRows();
  jumpTo("images");
}

// ---- Filters ---------------------------------------------------------------

const KINDS = [
  ["desktop", "Desktop"],
  ["gaming", "Gaming and handhelds"],
  ["server", "Server and homelab"],
  ["boards", "Raspberry Pi and other boards"],
  ["security", "Security and privacy"],
  ["rescue", "Rescue and tools"],
  ["windows", "Windows"],
  ["other", "Other"],
];

const ARCHES = [
  ["x86_64", "64-bit (x86_64)"],
  ["x86", "32-bit (x86)"],
  ["arm64", "ARM 64-bit"],
  ["arm", "ARM 32-bit"],
  ["multi", "Multi"],
];

const SHOW = [
  ["updates", "Updates ready"],
  ["favorites", "Favourites"],
  ["older", "Older versions"],
  ["caution", "Worth knowing (⚠)"],
];

// renderFilters fills the Filter menu with what this folder actually holds,
// and shows every filter that is on as a chip above the list.
function renderFilters() {
  const box = $("filter-items");
  box.replaceChildren();
  if (!state.report) return;
  const items = state.report.items;
  const has = (pick) => items.some(pick);

  const group = (title, boxes) => boxes.length
    ? el("div", { class: "filter-group" }, el("div", { class: "filter-title" }, title), boxes)
    : null;

  box.append(
    group("Show", SHOW.filter(([key]) => {
      if (key === "updates") return has((it) => it.status === "update available");
      if (key === "favorites") return has((it) => it.entry && (state.tracks[it.entry] || {}).starred);
      if (key === "older") return has((it) => it.older);
      return has((it) => cautionOf(it));
    }).map(([key, label]) => filterBox(label, view.show[key], (on) => {
      view.show[key] = on;
    }))),
    group("Kind", KINDS.filter(([kind]) => has((it) => (it.category || "other") === kind))
      .map(([kind, label]) => filterBox(label, view.kinds.includes(kind), (on) => {
        view.kinds = on ? [...view.kinds, kind] : view.kinds.filter((k) => k !== kind);
      }))),
    group("Architecture", ARCHES.filter(([arch]) => has((it) => it.arch === arch))
      .map(([arch, label]) => filterBox(label, view.arches.includes(arch), (on) => {
        view.arches = on ? [...view.arches, arch] : view.arches.filter((a) => a !== arch);
      }))));

  const count = activeFilters().length;
  const badge = $("filter-count");
  badge.hidden = count === 0;
  badge.textContent = count;
  renderChips();
}

function filterBox(label, checked, set) {
  return el("label", { class: "check" },
    el("input", {
      type: "checkbox",
      checked: checked || undefined,
      onchange: (e) => {
        set(e.target.checked);
        saveView();
        renderFilters();
        renderRows();
      },
    }),
    label);
}

// activeFilters lists what is filtering the list right now: each one gets a
// chip, so a short list always says why it is short.
function activeFilters() {
  const chips = [];
  if (view.status) {
    chips.push({ label: statusWord(view.status), off: () => { view.status = ""; } });
  }
  for (const [key, label] of SHOW) {
    if (view.show[key]) chips.push({ label, off: () => { view.show[key] = false; } });
  }
  for (const [kind, label] of KINDS) {
    if (view.kinds.includes(kind)) chips.push({ label, off: () => { view.kinds = view.kinds.filter((k) => k !== kind); } });
  }
  for (const [arch, label] of ARCHES) {
    if (view.arches.includes(arch)) chips.push({ label, off: () => { view.arches = view.arches.filter((a) => a !== arch); } });
  }
  const query = $("search").value.trim();
  if (query) chips.push({ label: `“${query}”`, off: () => { $("search").value = ""; } });
  return chips;
}

function renderChips() {
  const box = $("chips");
  const chips = activeFilters();
  box.hidden = chips.length === 0;
  box.replaceChildren(
    ...chips.map((chip) => el("button", {
      type: "button", class: "chip-off", title: "Stop filtering by this",
      onclick: () => {
        chip.off();
        saveView();
        renderFilters();
        renderRows();
      },
    }, chip.label, el("span", { class: "x", "aria-hidden": "true" }, "✕"))),
    chips.length > 1
      ? el("button", { type: "button", class: "linkish", onclick: clearFilters }, "Clear all")
      : null);
}

function clearFilters() {
  view.status = "";
  view.kinds = [];
  view.arches = [];
  for (const [key] of SHOW) view.show[key] = false;
  $("search").value = "";
  saveView();
  renderFilters();
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
    return (!view.status || item.status === view.status) &&
      (!query || haystack.includes(query)) &&
      (!view.kinds.length || view.kinds.includes(item.category || "other")) &&
      (!view.arches.length || view.arches.includes(item.arch)) &&
      (!view.show.updates || item.status === "update available") &&
      (!view.show.favorites || starred) &&
      (!view.show.older || item.older) &&
      (!view.show.caution || cautionOf(item));
  }));

  for (const item of items) rows.append(renderRow(item));
  // Like the key on a menu: only there when something in the list has the mark.
  $("legend").hidden = !items.some((item) => cautionOf(item));

  const total = state.report.items.length;
  $("shown").textContent = items.length === total ? plural(total, "image") : `${items.length} of ${plural(total, "image")}`;

  empty.hidden = items.length > 0;
  empty.replaceChildren();
  if (items.length === 0) {
    if (total === 0) {
      empty.append("No images found in this folder.");
    } else {
      empty.append(
        `None of the ${plural(total, "image")} here match ${activeFilters().map((c) => c.label).join(", ")}.`,
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
  ["col-image", "name", "By name, A to Z"],
  ["col-version", "version", "Newest version here first"],
  ["col-status", "attention", "Most urgent first"],
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
    placeMenu(menu);
  });
  return menu;
}

// placeMenu pins an open menu to the window, so the table's scroll box can't
// cut it off, and opens it upwards when there's no room below.
function placeMenu(menu) {
  const items = menu.querySelector(".menu-items");
  const anchor = menu.querySelector("summary").getBoundingClientRect();
  const floor = innerHeight - ($("dock").hidden ? 8 : $("dock").offsetHeight + 8);
  items.classList.add("pinned");
  items.style.right = `${Math.max(8, innerWidth - anchor.right)}px`;
  const below = anchor.bottom + 4;
  items.style.top = below + items.offsetHeight > floor
    ? `${Math.max(8, anchor.top - items.offsetHeight - 4)}px`
    : `${below}px`;
}

function closeMenus() {
  for (const open of document.querySelectorAll("details.menu[open]")) open.open = false;
}

function renderRow(item) {
  const track = (item.entry && state.tracks[item.entry]) || {};
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
    disabled: !item.entry || scanning(),
    onclick: () => setTrack(item.entry, { starred: !track.starred }),
  }, track.starred ? "★" : "☆");

  const statusCell = el("td", {},
    el("span", { class: `pill ${STATUS_CLASS[item.status] || "s-muted"}`, title: statusHelp(item.status) },
      statusWord(item.status)),
    item.eol && item.status !== "EOL"
      ? el("span", { class: "pill s-eol", title: statusHelp("EOL") }, "Old release")
      : null);

  const name = item.page
    ? el("a", { href: item.page, target: "_blank", rel: "noopener noreferrer", title: "Open the download page" }, item.name)
    : item.name;
  const caution = cautionOf(item);
  const imageCell = el("td", {}, el("div", { class: "image-cell" },
    logoTile(item),
    el("div", { class: "image-text" },
      // The name may wrap; the badges sit on their own line underneath so a
      // wrapped name never leaves one dangling at the end of it.
      el("div", { class: "name-line" },
        el("span", { class: "name" }, name),
        caution ? el("span", { class: "caution", title: caution, "aria-label": `Worth knowing: ${caution}` }, "⚠") : null),
      item.arch ? el("div", { class: "meta-line" }, el("span", { class: "arch" }, item.arch)) : null,
      item.note ? el("div", { class: "note" }, item.note) : null)));

  // The file, its size and when it arrived go under the name: one line each
  // instead of four columns.
  const under = [];
  if (item.path) {
    under.push(el("span", { class: "file", title: item.path }, breakable(item.path)));
    if (item.size) under.push(el("span", {}, formatBytes(item.size)));
    if (item.added) {
      under.push(el("span", { title: new Date(item.added).toLocaleString() },
        `${item.placed ? "Updated" : "Added"} ${shortDate(item.added)}`));
    }
  } else {
    under.push(el("span", { class: "muted" }, "Not in this folder"));
  }
  imageCell.querySelector(".image-text").append(el("div", { class: "under" }, join(under, " · ")));

  // "22.04 → 24.04" says more in one column than two ever did.
  const newer = item.latest && item.status === "update available";
  const versionCell = el("td", { class: "version-cell" },
    item.version || (item.latest ? "" : "–"),
    newer ? el("span", { class: "latest-new", title: item.latest_file || "" }, `${item.version ? " → " : ""}${item.latest}`) : null);

  const actions = [];
  if (item.entry && item.updates === "download" && item.status === "update available") {
    actions.push(jobButton(item.entry, el("button", {
      type: "button", class: "btn small primary",
      title: `Download ${item.latest || "the newest version"} and put it in this folder`,
      onclick: () => updateItem(item),
    }, "Update"), true));
  } else if (item.path && !item.entry) {
    actions.push(el("button", {
      type: "button", class: "btn small primary", disabled: scanning(),
      title: "Let isoshelf work out what this file is",
      "aria-label": `Identify ${item.path}`,
      onclick: () => openIdentify(item),
    }, "What is this?"));
  }
  actions.push(el("button", {
    type: "button", class: "btn small", "aria-label": `Everything about ${item.name}`,
    title: "Links, settings and everything else about this image",
    onclick: () => openDetails(item),
  }, "Details"));

  // The whole row opens the details panel; the buttons in it don't.
  const row = el("tr", {
    class: detailsKey(item) === detailsOpen ? "row picked" : "row",
    onclick: (e) => {
      if (e.target.closest("button, a, input, label, summary")) return;
      openDetails(item);
    },
  },
    el("td", { class: "col-star" }, star),
    imageCell,
    versionCell,
    statusCell,
    el("td", { class: "row-actions" }, actions));
  return row;
}

// join puts a separator between parts, the way a sentence would.
function join(parts, separator) {
  return parts.flatMap((part, i) => (i ? [separator, part] : [part]));
}

// breakable lets a long filename wrap after its separators — the _ - and .
// between words — instead of in the middle of one.
function breakable(text) {
  const parts = [];
  for (const piece of text.split(/(?<=[_\-.])/)) parts.push(piece, el("wbr"));
  parts.pop();
  return parts;
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
  // An open menu is pinned to the window; scrolling would leave it behind.
  document.addEventListener("scroll", closeMenus, true);
  window.addEventListener("resize", closeMenus);
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
  $("dock-toggle").addEventListener("click", () => {
    dockOpen = !dockOpen;
    renderDock();
  });
  $("dock-stop").addEventListener("click", stopDownloads);
  $("dock-clear").addEventListener("click", clearFinished);
  document.addEventListener("keydown", (e) => {
    if (e.key !== "Escape" || document.querySelector("dialog[open]")) return;
    if (detailsOpen) {
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

// ---- Details panel ---------------------------------------------------------

// Everything about one image sits in a panel beside the list: where it came
// from, what happens to old copies, its links, and what you can do with it.
// The row itself stays short.
let detailsOpen = null;

const KIND_LABEL = Object.fromEntries(KINDS);

function detailsKey(item) {
  return item.path || item.entry || item.name;
}

function openDetails(item) {
  detailsOpen = detailsKey(item);
  renderDetails();
  renderRows();
  $("details-close").focus();
}

function closeDetails() {
  detailsOpen = null;
  $("details").hidden = true;
  renderRows();
}

function renderDetails() {
  const panel = $("details");
  const item = detailsOpen && state.report
    ? state.report.items.find((it) => detailsKey(it) === detailsOpen)
    : null;
  if (!item) {
    detailsOpen = null;
    panel.hidden = true;
    return;
  }
  panel.hidden = false;
  $("details-logo").replaceChildren(logoTile(item));
  $("details-name").textContent = item.name;
  $("details-sub").textContent = [item.arch, KIND_LABEL[item.category], item.family]
    .filter(Boolean).join(" · ");

  const track = (item.entry && state.tracks[item.entry]) || {};
  const parts = [];
  parts.push(detailRow("Status", [
    el("div", {}, el("span", { class: `pill ${STATUS_CLASS[item.status] || "s-muted"}` }, statusWord(item.status))),
    el("div", { class: "muted" }, statusHelp(item.status)),
    item.note ? el("div", { class: "note" }, item.note) : null,
    cautionOf(item) ? el("div", { class: "note" }, `⚠ ${cautionOf(item)}`) : null,
  ]));

  if (item.path) {
    parts.push(detailRow("File", [
      el("div", { class: "file" }, breakable(item.path)),
      el("div", { class: "muted" }, join([
        item.size ? formatBytes(item.size) : null,
        item.kind && item.kind !== "unknown" ? item.kind : null,
        item.added ? `${item.placed ? "updated" : "added"} ${shortDate(item.added)}` : null,
      ].filter(Boolean), " · ")),
    ]));
  }

  parts.push(detailRow("Version", [
    el("div", {}, item.version || el("span", { class: "muted" }, "not known")),
    item.latest ? el("div", { class: "muted" }, `newest published: ${item.latest}`) : null,
    versionField(item),
  ]));

  if (item.entry && item.updates === "download") {
    parts.push(detailRow("When an update arrives", [choiceField(item, track)]));
  }

  const links = linkList(item);
  if (links.length) parts.push(detailRow("Links", el("div", { class: "detail-links" }, links)));

  const buttons = [];
  if (item.entry && item.updates === "download" && item.status === "update available") {
    buttons.push(jobButton(item.entry, el("button", {
      type: "button", class: "btn primary",
      onclick: () => updateItem(item),
    }, "Update"), true));
  }
  if (item.path && !item.entry) {
    buttons.push(el("button", {
      type: "button", class: "btn primary", disabled: scanning(),
      onclick: () => openIdentify(item),
    }, "What is this?"));
  }
  if (item.path && item.assigned) {
    buttons.push(el("button", {
      type: "button", class: "btn", disabled: scanning(),
      title: "You told isoshelf what this file is. Change that.",
      onclick: () => openIdentify(item),
    }, "Not right?"));
  }
  if (item.path) {
    buttons.push(el("button", {
      type: "button", class: "btn", disabled: scanning(),
      onclick: () => removeItem(item),
    }, "Remove…"));
  }
  parts.push(el("div", { class: "details-buttons" }, buttons));
  $("details-body").replaceChildren(...parts.filter(Boolean));
}

function detailRow(label, children) {
  return el("div", { class: "detail-row" },
    el("div", { class: "detail-label" }, label),
    el("div", { class: "detail-value" }, children));
}

// versionField lets you say which version a file is when its name doesn't
// say and the project publishes nothing to compare against - Hiren's BootCD,
// for one. isoshelf remembers it until the file itself changes.
function versionField(item) {
  if (!item.path || !item.entry || item.version) return null;
  const input = el("input", {
    type: "text", class: "version-input", maxlength: "40", spellcheck: "false",
    placeholder: "1.0.8", "aria-label": `Version of ${item.path}`,
  });
  return el("div", { class: "version-set" },
    input,
    el("button", {
      type: "button", class: "btn small", disabled: scanning(),
      onclick: async () => {
        const version = input.value.trim();
        if (!version) return;
        try {
          await api("POST", "/api/identify", { path: item.path, entry: item.entry, version });
        } catch (err) {
          showNotice(err.message, true);
          return;
        }
        flashNotice(`${item.path} is version ${version}.`);
        await refresh();
      },
    }, "Save"),
    el("div", { class: "muted" }, "isoshelf can't tell which version this file is. If you know, say so here."));
}

// sameName is true for images whose filename never changes, where the new
// file lands on top of the old one and keeping both is impossible.
function sameName(item) {
  return Boolean(item.latest_file && item.path && item.path.split("/").pop() === item.latest_file);
}

function choiceFor(item) {
  const track = (item.entry && state.tracks[item.entry]) || {};
  let choice = track.old_files || (track.keep_old ? "keep" : "replace");
  if (choice === "keep" && sameName(item)) choice = "archive";
  return choice;
}

const CHOICES = [
  ["replace", "Replace the old file"],
  ["archive", "Archive the old file (undo any time)"],
  ["keep", "Keep both"],
];

const CHOICE_WORD = {
  replace: "replaces the old file",
  archive: "archives the old file",
  keep: "keeps both",
};

// choiceField is the one decision each image carries: what happens to the
// copy it replaces. It is saved the moment it changes, so an update never
// has to stop and ask.
function choiceField(item, track) {
  const current = choiceFor(item);
  const select = el("select", {
    "aria-label": `What happens to old ${item.name} files`,
    disabled: scanning(),
    onchange: (e) => setTrack(item.entry, { old_files: e.target.value }),
  }, CHOICES.filter(([value]) => value !== "keep" || !sameName(item))
    .map(([value, label]) => el("option", { value, selected: value === current || undefined }, label)));
  return el("div", {},
    select,
    el("div", { class: "muted" }, sameName(item)
      ? "This image always has the same filename, so the new one takes its place. Archiving keeps the old one in this folder until you empty the archive."
      : "Used for every update of this image from now on."));
}

function linkList(item) {
  return [
    ["Download page", item.page],
    ["Website", item.site],
    ["Forum", item.forum],
    ["Release notes", item.release],
    problemLink(item),
  ].filter((link) => link && link[1])
    .map(([label, url]) => el("a", { href: url, target: "_blank", rel: "noopener noreferrer" }, label));
}

// ---- The checklist ---------------------------------------------------------

// One dialog for every "which of these?" question: updating several images,
// clearing older versions. It lists what will happen to each, lets you untick
// anything, and adds up what it frees or downloads.
function pickFiles({ title, text, rows, actions, sizeLabel }) {
  const dialog = $("pick");
  $("pick-title").textContent = title;
  $("pick-text").textContent = text;
  const list = $("pick-list");
  const total = $("pick-total");
  const boxes = new Map();

  const tally = () => {
    const chosen = rows.filter((row) => boxes.get(row.id).checked);
    const bytes = chosen.reduce((sum, row) => sum + (row.size || 0), 0);
    total.textContent = `${plural(chosen.length, "image")} chosen${bytes ? ` · ${sizeLabel || "about"} ${formatBytes(bytes)}` : ""}`;
  };

  list.replaceChildren(...rows.map((row) => {
    const box = el("input", { type: "checkbox", checked: row.checked !== false || undefined, onchange: tally });
    boxes.set(row.id, box);
    return el("li", {},
      el("label", { class: "pick-item" },
        box,
        el("span", { class: "info" },
          el("span", { class: "name" }, row.name),
          row.detail ? el("span", { class: "kind" }, row.detail) : null),
        row.note ? el("span", { class: "muted pick-note" }, row.note) : null));
  }));
  tally();

  return new Promise((resolve) => {
    $("pick-actions").replaceChildren(...actions.map((action) => el("button", {
      type: "button", class: `btn ${action.primary ? "primary" : ""}`,
      onclick: () => {
        const chosen = rows.filter((row) => boxes.get(row.id).checked).map((row) => row.id);
        dialog.close();
        resolve(chosen.length ? { action: action.value, ids: chosen } : null);
      },
    }, action.label)));
    dialog.addEventListener("close", () => resolve(null), { once: true });
    dialog.showModal();
  });
}

// reviewOlder lists the older versions and clears the ones you tick.
async function reviewOlder(older) {
  const newest = {};
  for (const item of state.report.items) {
    if (item.entry && !item.older && item.path) newest[item.entry] = item.path.split("/").pop();
  }
  const answer = await pickFiles({
    title: "Older versions you could clear",
    text: "You have a newer version of each of these. Untick anything you want to keep.",
    sizeLabel: "freeing",
    rows: older.map((item) => ({
      id: item.path,
      size: item.size,
      name: item.name,
      detail: `${item.path}${item.size ? ` · ${formatBytes(item.size)}` : ""}`,
      note: newest[item.entry] ? `newer here: ${newest[item.entry]}` : "",
    })),
    actions: [
      { label: "Archive them", value: "move-aside", primary: true },
      { label: "Delete them", value: "delete" },
    ],
  });
  if (!answer) return;
  try {
    state = await api("POST", "/api/remove", { paths: answer.ids, how: answer.action });
    catalog = null;
  } catch (err) {
    showNotice(err.message, true);
    return;
  }
  flashNotice(answer.action === "delete"
    ? `${plural(answer.ids.length, "older version")} deleted.`
    : `${plural(answer.ids.length, "older version")} archived, under Archive on this page.`);
  if (state.run && state.run.kind === "update") {
    render();
    return;
  }
  await start("scan");
}

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

"use strict";

// The download queue: the dock at the bottom of the page, what is running,
// what is waiting, what has finished, and the buttons that reorder it.

// ---- Downloads -------------------------------------------------------------

// Downloads go in the queue's order, which can be changed - like a game
// launcher's download queue - and more than one runs at a time (v0.10.0): two
// from different servers, or more under a speed limit, never two from one
// server (the rules are in internal/web/schedule.go). The bar at the bottom of
// the page shows what is running, a row each when it is more than one, and
// opening it shows the rest.

let dockOpen = false;
// dockDrawn is what the list last showed, so it is only rebuilt when that
// changes; dragging is the id of the download being dragged, if one is.
let dockDrawn = "";
let dragging = null;
// dockRuns holds each running download's line and bar in the open list, by
// the id of the download.
let dockRuns = new Map();
// barRows is the same for the rows on the bar itself: which downloads they
// were drawn for, and each one's line and bar.
let barRows = { key: "", rows: new Map() };

function downloads() {
  return (state && state.downloads) || { running: [], queued: [], finished: [] };
}

// jobFor says where an image is in the downloads: downloading now, waiting
// (and at which place), or how it ended. Null when it isn't there.
function jobFor(entry) {
  const d = downloads();
  const going = d.running.find((job) => job.entry === entry);
  if (going) return { where: "running", job: going };
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
  return at.where === "running" || at.where === "queued" || (at.where === "done" && arrivedSinceScan(at.job));
}

function arrivedSinceScan(job) {
  return !state.updated_at || new Date(job.at) > new Date(state.updated_at);
}

// pendingBytes is roughly how much the queue still has to write.
function pendingBytes() {
  const d = downloads();
  let bytes = d.queued.reduce((sum, job) => sum + (job.size || 0), 0);
  for (const cur of d.running) {
    bytes += cur.total ? Math.max(cur.total - cur.done, 0) : cur.size || 0;
  }
  return bytes;
}

// downloadProgress describes a download that is running, in words and as a
// fraction (null when there's no telling). With none given, the one that has
// been running longest.
function downloadProgress(run = downloads().running[0]) {
  // The download carries its own progress, so the page can draw it and a
  // scan's card at the same time.
  if (!run) return { text: "Starting…", fraction: null, short: "Starting…" };
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
    // Copying from another isoshelf is worth seeing while it happens: it is
    // the difference between a minute and an hour, and until now the only
    // way to tell was the speed.
    if (run.from) text += ` · from ${run.from}`;
    return { text, fraction, short: `Downloading ${pct}` };
  }
  if (run.stage === "downloading") {
    return { text: run.from ? `Downloading from ${run.from}…` : "Downloading…", fraction: null, short: "Downloading…" };
  }
  if (run.stage === "verifying") {
    return {
      text: "Verifying it against the published checksum…",
      fraction: run.total > 0 ? run.done / run.total : null,
      short: "Verifying…",
    };
  }
  if (run.stage === "placing") return { text: "Putting it in place…", fraction: 1, short: "Almost done" };
  return { text: "Finding the download…", fraction: null, short: "Starting…" };
}

// downloadRate is how fast a download is going, in bytes a second. isoshelf
// measures it for each one (more than one at a time makes a single reading
// in the page meaningless) and says so in the state.
function downloadRate(run) {
  return run.speed || 0;
}

function duration(seconds) {
  if (seconds < 90) return "under 2 min";
  const minutes = Math.round(seconds / 60);
  if (minutes < 90) return `about ${minutes} min`;
  return `about ${Math.round(minutes / 60)} h`;
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
    `Stops the download running now${waiting ? `, and takes ${plural(waiting, "image")} off the queue` : ""}. What has downloaded so far is kept, so adding it again continues from there.`,
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
    case "running":
      return el("button", {
        type: "button", class: "btn small job-state", "data-downloading": entry,
        title: "Downloading now. Click to see the downloads.",
        onclick: openDock,
      }, downloadProgress(at.job).short);
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
        return el("span", { class: "job-done" }, icon("check-16"), at.job.update ? "Updated" : "Added");
      }
      return idle;
    default:
      if (idle instanceof HTMLButtonElement && !idle.disabled) {
        idle.textContent = at.where === "stopped" ? "Resume" : "Try again";
        idle.title = at.job.message || idle.title;
      }
      return idle;
  }
}


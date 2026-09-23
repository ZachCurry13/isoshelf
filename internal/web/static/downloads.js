"use strict";

// The download queue: the dock at the bottom of the page, what is running,
// what is waiting, what has finished, and the buttons that reorder it.

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
    const cur = d.current;
    bytes += cur.total ? Math.max(cur.total - cur.done, 0) : cur.size || 0;
  }
  return bytes;
}

// downloadProgress describes the download that is running, in words and as a
// fraction (null when there's no telling).
function downloadProgress() {
  // The download carries its own progress, so the page can draw it and a
  // scan's card at the same time.
  const run = downloads().current;
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

function downloadRate(run) {
  const id = run.id;
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
        title: "Stop this one and start the next. What has downloaded so far is kept.",
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
  if (job.from) what += ` from ${job.from}`;
  // Done, but with something to know: a file nothing could check.
  if (job.outcome === "done" && job.message) {
    [mark, tone] = ["!", "s-warn"];
    what += `. ${job.message}`;
  }
  if (job.outcome === "failed") what = job.conflict ? job.message : `Failed: ${job.message}`;
  if (job.conflict) [mark, tone] = ["?", "s-update"];
  if (job.outcome === "stopped") what = "Stopped. What has downloaded so far is kept, so Resume continues from there.";
  // Only the latest try gets the button, and not once it is queued again.
  const again = job.outcome !== "done" && jobFor(job.entry).job.id === job.id;
  return el("li", { class: "dock-item finished" },
    el("span", { class: `pill ${tone} mark`, "aria-hidden": "true" }, mark),
    el("div", { class: "info" },
      el("div", { class: "name" }, job.name),
      el("div", { class: "kind" }, what, job.at ? ` · ${timeAgo(job.at)}` : "")),
    again ? againButtons(job) : null);
}

// againButtons is what to do about a download that didn't finish. A name
// clash is a question, not a failure, so it gets the answers themselves
// instead of a "Try again" that would fail the same way. Doing nothing is
// always allowed: nothing has changed in the folder either way.
function againButtons(job) {
  if (!job.conflict) {
    const again = el("button", {
      type: "button", class: "btn small",
      title: job.outcome === "stopped"
        ? "Continue from where it stopped"
        : "Put it back on the queue and try again",
      onclick: () => retryJob(job),
    }, job.outcome === "stopped" ? "Resume" : "Try again");
    if (job.outcome !== "failed") return again;
    return el("div", { class: "conflict-choices" }, again,
      el("button", {
        type: "button", class: "btn small",
        title: "Open a bug report with the details filled in",
        onclick: () => reportProblem(`${job.name} wouldn't download`, job.message),
      }, "Report this"));
  }
  // Three real answers now. Keeping both used to be refused for these
  // images, because the new file wanted a name the old one already had; the
  // new one carries its version in its name instead, and nothing already in
  // the folder is touched.
  return el("div", { class: "conflict-choices" },
    el("button", {
      type: "button", class: "btn small",
      title: "Download it under a name with its version in it, and leave your existing file alone",
      onclick: () => queueDownload(job.entry, "keep"),
    }, "Keep both"),
    el("button", {
      type: "button", class: "btn small",
      title: "Download it and move the old file to the archive, where you can restore it",
      onclick: () => queueDownload(job.entry, "move-aside"),
    }, "Archive the old one"),
    el("button", {
      type: "button", class: "btn small",
      title: "Download it and delete the old file once the new one is verified",
      onclick: () => queueDownload(job.entry, "delete"),
    }, "Replace it"));
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
      count("failed") && `${count("failed")} failed`,
      count("stopped") && `${count("stopped")} stopped`,
    ].filter(Boolean);
    text = `Finished: ${parts.join(", ")}.`;
    if (scanning()) text += " Looking at the folder again…";
  }
  $("dock-text").textContent = text;
  // With thirty images queued, "30 waiting" doesn't answer the question
  // anyone actually has, which is how long and how much room.
  const left = pendingBytes();
  $("dock-waiting").textContent = d.queued.length
    ? `${d.queued.length} waiting${left ? ` · ${formatBytes(left)} to download` : ""}`
    : "";

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
        idle.textContent = at.where === "stopped" ? "Resume" : "Try again";
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

// topDialog is the modal dialog on top, if one is open.
//
// A dialog shown with showModal() is drawn in the browser's top layer, above
// everything else on the page. A message written into the page behind it is
// therefore invisible - which is how a refused action inside the identify
// dialog came to look like a button that did nothing at all. Reported by the
// maintainer as "I clicked That's it and nothing happened", and reproduced:
// the message was there, underneath.
function topDialog() {
  const open = document.querySelectorAll("dialog[open]");
  return open.length ? open[open.length - 1] : null;
}

// noticeIn is the message slot inside a dialog, made the first time one is
// needed and taken away when the dialog closes, so it never reappears stale
// the next time the dialog is opened.
function noticeIn(dialog) {
  let box = dialog.querySelector(".dialog-notice");
  if (!box) {
    box = el("div", { class: "notice dialog-notice", role: "status" });
    (dialog.querySelector("form") || dialog).prepend(box);
    dialog.addEventListener("close", () => box.remove(), { once: true });
  }
  return box;
}

function showNotice(message, isError) {
  const dialog = topDialog();
  // The page's own notice is always set, so the message is still there after
  // a dialog closes. What changes is whether anyone can see it right now.
  fillNotice($("notice"), message, isError);
  if (dialog) fillNotice(noticeIn(dialog), message, isError);
}

function fillNotice(notice, message, isError) {
  notice.replaceChildren(message);
  notice.classList.toggle("error", Boolean(isError));
  notice.hidden = false;
  // Something went wrong is exactly the moment to offer a way to say so.
  if (isError) {
    notice.append(" ", el("button", {
      type: "button", class: "linkish",
      onclick: () => reportProblem("Something went wrong in isoshelf", message),
    }, "Report this"));
  }
}

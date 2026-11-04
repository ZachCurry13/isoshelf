"use strict";

// The open list of downloads above the bar: what is running, waiting and
// finished, with the buttons that reorder it. The bar itself is in
// dockprogress.js.

function renderDock() {
  const d = downloads();
  const any = Boolean(d.running.length || d.queued.length || d.finished.length);
  $("dock").hidden = !any;
  document.body.classList.toggle("has-dock", any);
  if (!any) {
    dockOpen = false;
    return;
  }
  $("dock-panel").hidden = !dockOpen;
  $("dock-toggle").textContent = dockOpen ? "Hide" : "Show";
  $("dock-toggle").setAttribute("aria-expanded", String(dockOpen));
  $("dock-stop").hidden = !(d.running.length || d.queued.length);
  $("dock-clear").hidden = !d.finished.length;
  renderDockProgress();

  // Never rebuild the list under a drag, or while it hasn't changed: the
  // button that has focus would go with it.
  const key = JSON.stringify([dockOpen, d.running.map((job) => job.id),
    d.queued.map((job) => job.id), d.finished.map((job) => [job.id, job.outcome])]);
  if (dragging !== null || key === dockDrawn) return;
  dockDrawn = key;

  const list = $("dock-list");
  const focused = rememberFocus();
  list.replaceChildren();
  dockRuns = new Map();
  if (!dockOpen) return;
  if (d.running.length) {
    list.append(el("li", { class: "dock-heading" }, "Downloading"));
    for (const run of d.running) {
      const going = {
        text: el("div", { class: "kind" }, downloadProgress(run).text),
        fill: el("div", { class: "bar-fill" }),
      };
      dockRuns.set(run.id, going);
      list.append(el("li", { class: "dock-item current" },
        logoTile(entryInfo(run)),
        el("div", { class: "info" },
          el("div", { class: "name" }, run.name),
          going.text,
          el("div", { class: "bar" }, going.fill)),
        el("button", {
          type: "button", class: "btn small",
          title: "Stop this one. What has downloaded so far is kept, and anything waiting that can go takes its place.",
          "aria-label": `Stop downloading ${run.name}`,
          onclick: () => dropJob(run),
        }, "Stop")));
    }
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
    el("span", { class: "grip", "aria-hidden": "true" }, icon("grabber-16")),
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
      }, icon("arrow-up-16")),
      el("button", {
        type: "button", class: "btn small icon", disabled: i === count - 1,
        title: "Move down", "aria-label": `Move ${job.name} down`, "data-focus": `${job.id}:down`,
        onclick: () => moveJob(job, i + 1),
      }, icon("arrow-down-16")),
      el("button", {
        type: "button", class: "btn small icon", disabled: i === 0,
        title: "Next in line", "aria-label": `Download ${job.name} next`, "data-focus": `${job.id}:top`,
        onclick: () => moveJob(job, 0),
      }, icon("move-to-top-16")),
      el("button", {
        type: "button", class: "btn small icon",
        title: "Take it off the queue", "aria-label": `Take ${job.name} off the queue`, "data-focus": `${job.id}:drop`,
        onclick: () => dropJob(job),
      }, icon("x-16"))));

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
  done: ["check-16", "s-ok"],
  failed: ["x-16", "s-bad"],
  stopped: ["stop-16", "s-muted"],
};

function finishedItem(job) {
  let [mark, tone] = OUTCOME[job.outcome] || ["", "s-muted"];
  let what = job.update ? "Updated" : "Added";
  if (job.from) what += ` from ${job.from}`;
  // Done, but with something to know: a file nothing could check.
  if (job.outcome === "done" && job.message) {
    [mark, tone] = ["alert-16", "s-warn"];
    what += `. ${job.message}`;
  }
  if (job.outcome === "failed") what = job.conflict ? job.message : `Failed: ${job.message}`;
  if (job.conflict) [mark, tone] = ["question-16", "s-update"];
  if (job.outcome === "stopped") what = "Stopped. What has downloaded so far is kept, so Resume continues from there.";
  // Only the latest try gets the button, and not once it is queued again.
  const again = job.outcome !== "done" && jobFor(job.entry).job.id === job.id;
  return el("li", { class: "dock-item finished" },
    el("span", { class: `pill ${tone} mark`, "aria-hidden": "true" }, mark ? icon(mark) : null),
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

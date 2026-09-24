"use strict";

// Adding a file from this computer: dragged anywhere onto the page, or picked
// with the file chooser.
//
// Unlike a download, this doesn't go through the server's queue - the file is
// the body of one request, so the browser is the only thing that knows how
// far it has got. That is why the list below is kept here rather than read
// from /api/state like the downloads dock.

// waiting is the files still to go; sending is the one going now. One at a
// time: these are gigabytes over a bus the drive shares with everything else.
let waiting = [];
let sending = null;
// done is what finished or failed, newest first, so the page can say so.
let sent = [];
// dragDepth counts dragenter minus dragleave. Dragging over a child element
// fires dragleave on the parent, so a plain boolean flickers.
let dragDepth = 0;

function wireUpload() {
  $("upload-choose").addEventListener("click", () => $("upload-input").click());
  $("upload-input").addEventListener("change", (e) => {
    addFiles(e.target.files);
    e.target.value = ""; // so choosing the same file twice still counts
  });

  // The whole window is the drop target: a page this long would otherwise
  // mean scrolling to a small rectangle with a file held in one hand.
  window.addEventListener("dragenter", (e) => {
    if (!hasFiles(e)) return;
    e.preventDefault();
    dragDepth++;
    showOverlay(true);
  });
  window.addEventListener("dragover", (e) => {
    if (!hasFiles(e)) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = "copy";
  });
  window.addEventListener("dragleave", (e) => {
    if (!hasFiles(e)) return;
    dragDepth = Math.max(0, dragDepth - 1);
    if (dragDepth === 0) showOverlay(false);
  });
  window.addEventListener("drop", (e) => {
    if (!hasFiles(e)) return;
    // Without this the browser leaves the page and opens the file itself.
    e.preventDefault();
    dragDepth = 0;
    showOverlay(false);
    addFiles(e.dataTransfer.files);
  });
}

function hasFiles(e) {
  return Boolean(e.dataTransfer) && Array.from(e.dataTransfer.types || []).includes("Files");
}

function showOverlay(on) {
  const overlay = $("drop-overlay");
  // Nowhere to put a file until a folder is chosen, so don't invite one.
  overlay.hidden = !on || !state || !state.target;
}

function addFiles(list) {
  if (!state || !state.target) {
    showNotice("Choose a folder first, then add files to it.", true);
    return;
  }
  for (const file of list) waiting.push(file);
  renderUploads();
  sendNext();
}

// sendNext starts the next file if nothing is going.
function sendNext() {
  if (sending || !waiting.length) return;
  send(waiting.shift(), "");
}

// send puts one file on its way. replace is the answer about a file of the
// same name already in the folder: "" until the user has been asked.
function send(file, replace) {
  sending = { file, name: file.name, loaded: 0, total: file.size };
  renderUploads();

  const req = new XMLHttpRequest();
  sending.request = req;
  let url = `/api/upload?name=${encodeURIComponent(file.name)}`;
  if (replace) url += `&replace=${encodeURIComponent(replace)}`;
  req.open("POST", url);
  req.setRequestHeader("X-Isoshelf", "1");
  req.upload.addEventListener("progress", (e) => {
    if (!sending) return;
    sending.loaded = e.loaded;
    if (e.lengthComputable) sending.total = e.total;
    uploadRate(sending);
    renderUploads();
  });
  req.addEventListener("load", () => finish(file, req));
  req.addEventListener("error", () => finish(file, req, "The connection to isoshelf dropped. Nothing was added."));
  req.addEventListener("abort", () => finish(file, req, "Stopped. Nothing was added."));
  req.send(file);
}

async function finish(file, req, trouble) {
  sending = null;
  // Draw straight away: the row that was showing progress is finished with,
  // and what comes next may be a dialog the user sits in front of for a while.
  renderUploads();
  const answer = parseAnswer(req);

  if (!trouble && req.status >= 200 && req.status < 300) {
    record(file.name, "done", answer.replaced
      ? `Added. The file that was here is in the archive.`
      : "Added.");
    // The server scans once the file is in place; this picks that up.
    await refresh();
    sendNext();
    return;
  }

  // A name clash is a question, not a failure - the same one a download
  // asks, so it gets the same two answers and the same words.
  if (answer.conflict) {
    const choice = await ask(
      `${file.name} is already here`,
      `${answer.error} Adding it again can put the file you have in the archive, ` +
      `where you can restore it, or replace it for good.`,
      [
        { label: "Archive the old one", value: "move-aside", primary: true },
        { label: "Replace it", value: "delete" },
        { label: "Cancel", value: "" },
      ]);
    if (choice) {
      send(file, choice);
      return;
    }
    record(file.name, "stopped", "Canceled. Nothing in the folder changed.");
    await refresh();
    sendNext();
    return;
  }

  record(file.name, "failed", trouble || answer.error || `Failed (${req.status}).`);
  sendNext();
}

// parseAnswer reads the server's reply, which is JSON unless something went
// very wrong.
function parseAnswer(req) {
  try {
    return JSON.parse(req.responseText) || {};
  } catch {
    return {};
  }
}

function record(name, outcome, message) {
  sent.unshift({ name, outcome, message });
  if (sent.length > 10) sent.length = 10;
  renderUploads();
}

// stopSending abandons the file going now. The server writes into
// .isoshelf/incoming until the whole file is there, so an abandoned upload
// leaves the folder exactly as it was.
function stopSending() {
  if (sending && sending.request) sending.request.abort();
}

function clearSent() {
  sent = [];
  renderUploads();
}

// uploadRate works out how fast a file is going, the way the downloads dock
// does: measured over a second at least, and smoothed, so the number doesn't
// jump about with every chunk the browser hands over.
function uploadRate(s) {
  const now = Date.now();
  if (!s.mark) {
    s.mark = { loaded: s.loaded, at: now };
    return;
  }
  const seconds = (now - s.mark.at) / 1000;
  if (seconds < 1) return;
  const rate = (s.loaded - s.mark.loaded) / seconds;
  s.rate = s.rate ? s.rate * 0.7 + rate * 0.3 : rate;
  s.mark = { loaded: s.loaded, at: now };
}

function renderUploads() {
  const list = $("upload-list");
  if (!list) return;
  const rows = [];

  if (sending) {
    const fraction = sending.total ? sending.loaded / sending.total : null;
    let text = `${formatBytes(sending.loaded)} of ${formatBytes(sending.total)}`;
    if (fraction !== null) text += ` · ${Math.floor(fraction * 100)}%`;
    // The same words the downloads dock uses for the same thing.
    if (sending.rate > 0) {
      text += ` · ${formatBytes(sending.rate)}/s`;
      const left = (sending.total - sending.loaded) / sending.rate;
      if (left > 0 && left < 86400) text += ` · ${duration(left)} left`;
    }
    // The width is set on the element rather than in a style attribute: the
    // page's own content policy allows no inline styles.
    const fill = el("div", { class: "bar-fill" });
    fill.style.width = fraction === null ? "0" : `${Math.round(fraction * 100)}%`;
    rows.push(el("li", { class: "upload-item current" },
      el("div", { class: "info" },
        el("div", { class: "name" }, sending.name),
        el("div", { class: "kind" }, text),
        el("div", { class: "bar" }, fill)),
      el("button", {
        type: "button", class: "btn small",
        title: "Stop adding this file. Nothing in the folder changes.",
        onclick: stopSending,
      }, "Stop")));
  }

  for (const file of waiting) {
    rows.push(el("li", { class: "upload-item" },
      el("div", { class: "info" },
        el("div", { class: "name" }, file.name),
        el("div", { class: "kind" }, `Waiting · ${formatBytes(file.size)}`))));
  }

  for (const job of sent) {
    rows.push(el("li", { class: "upload-item" },
      el("div", { class: "info" },
        el("div", { class: "name" }, job.name),
        el("div", { class: "kind" }, job.message))));
  }

  if (sent.length && !sending && !waiting.length) {
    rows.push(el("li", { class: "upload-item" },
      el("button", { type: "button", class: "btn small", onclick: clearSent }, "Clear")));
  }
  list.replaceChildren(...rows);
}

// uploading says whether a file is on its way, for the parts of the page that
// must wait for one (switching folders, which the server refuses too).
function uploading() {
  return Boolean(sending) || waiting.length > 0;
}

"use strict";

// Copying from your server (v0.8.1, #62). In Add images, a catalog image
// your server has is marked "On your server" and copied from there, and the
// files the catalog doesn't know are in a folded list at the bottom. A copy
// is checked against the server's copy on the way in, and recorded as a copy
// - never as checked against what the project publishes.

// serverFiles is what the server has that this folder doesn't: null until
// asked, and asked again whenever the catalog is.
let serverFiles = null;
let serverError = "";
let serverAsking = null;

// loadServerFiles asks once per catalog load, and only when a server is set
// up. The answer can take a few seconds when the server is asleep, so the
// list draws without it and again when it comes.
async function loadServerFiles() {
  if (serverFiles || !state.peer || !state.peer.on) return;
  if (!serverAsking) {
    serverAsking = api("GET", "/api/peer/files").then((answer) => {
      serverFiles = answer.files || [];
      serverError = answer.error || "";
    }, (err) => {
      serverFiles = [];
      serverError = err.message;
    }).finally(() => {
      serverAsking = null;
      renderCatalog();
    });
  }
}

// forgetServerFiles is called whenever the catalog is asked for again.
function forgetServerFiles() {
  serverFiles = null;
}

// serverFileFor is the server's file of a catalog image, if it has one.
function serverFileFor(entryId) {
  return (serverFiles || []).find((f) => f.entry === entryId && f.image);
}

function serverMark(file) {
  return el("span", {
    class: "pill s-ok",
    title: `Your server has ${file.name}${file.version ? `, version ${file.version}` : ""}.`,
  }, "On your server");
}

function copyButton(file, entryId) {
  const button = el("button", {
    type: "button", class: "btn small primary",
    title: `Copy ${file.name} from your server, checked against your server's copy on the way. It never replaces a file that's already here.`,
    onclick: () => copyFromServer(file),
  }, "Copy");
  return entryId ? jobButton(entryId, button, false) : button;
}

async function copyFromServer(file) {
  try {
    await api("POST", "/api/peer/copy", { name: file.name, sha256: file.sha256 });
  } catch (err) {
    showNotice(err.message, true);
    return;
  }
  flashNotice(`Copying ${file.name} from your server. It shows in the downloads.`);
  await refresh();
}

// renderServerOnly is "Also on your server": files the catalog doesn't know,
// folded away at the bottom of Add images.
function renderServerOnly() {
  const box = $("server-only");
  const files = (serverFiles || []).filter((f) => !f.image);
  box.hidden = files.length === 0 && !serverError;
  if (box.hidden) return;
  $("server-only-count").textContent = serverError ? "" : plural(files.length, "file");
  $("server-only-list").replaceChildren(...(serverError
    ? [el("li", { class: "muted" }, serverError)]
    : files.map((f) => el("li", {},
      el("div", { class: "info" },
        el("div", { class: "info-line" }, el("span", { class: "name" }, f.name)),
        el("div", { class: "meta-line" }, el("span", { class: "muted" },
          [formatBytes(f.size), f.since ? `on your server since ${shortDate(f.since)}` : ""].filter(Boolean).join(" · ")))),
      copyButton(f)))));
}

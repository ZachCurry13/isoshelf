"use strict";

// isoshelf updating its own program. The top bar says a new version is out
// (a link to what changed) and, where isoshelf can replace itself, carries
// Update now; the same place then says how it is going. Nothing is downloaded
// until the button is pressed - the maintainer's choice, 2026-09-23.
//
// While isoshelf restarts, the page keeps asking, and once the new version
// answers it reloads, so the page it shows is the new version's page too.

// selfFrom is the version that was running when a restart began, and null
// when none is under way.
let selfFrom = null;

// selfUpdating says whether the page should keep asking often: an update is
// under way, or isoshelf is restarting into one.
function selfUpdating() {
  const stage = state && state.self_update && state.self_update.stage;
  return selfFrom !== null || stage === "downloading" || stage === "waiting" || stage === "restarting";
}

// selfArrived is called with every answer. It reloads the page once a new
// version answers, and stops waiting if the old one came back instead -
// which is what an update that couldn't start looks like.
function selfArrived() {
  const u = state.self_update || {};
  if (u.stage === "restarting" && selfFrom === null) selfFrom = state.version;
  if (selfFrom === null || u.stage === "restarting") return false;
  if (state.version !== selfFrom) {
    location.reload();
    return true;
  }
  selfFrom = null;
  return false;
}

function renderSelfUpdate() {
  const u = state.self_update || {};
  const latest = state.app_update && state.app_update.latest;
  const link = $("app-update");
  const going = u.stage && u.stage !== "failed";
  link.hidden = !latest || Boolean(going);
  if (latest) {
    link.textContent = `isoshelf ${latest} is available`;
    link.href = state.app_update.url;
    // Where it can't update itself, the link is all there is, and it says why.
    link.title = u.can ? "What's new in it" : u.why || "";
  }
  const box = $("self-update");
  box.replaceChildren(...selfUpdateParts(u, latest));
  box.hidden = box.childElementCount === 0;
  announceSelfUpdate(u);
}

function selfUpdateParts(u, latest) {
  const cancel = () => el("button", { type: "button", class: "btn small", onclick: cancelSelfUpdate }, "Cancel");
  switch (u.stage) {
    case "downloading":
      return [el("span", {}, `Downloading isoshelf ${latest || ""}… ${u.total ? Math.floor(100 * u.done / u.total) : 0}%`), cancel()];
    case "waiting":
      return [el("span", {}, `isoshelf ${latest || ""} is ready. Waiting for ${plural(u.waiting || 1, "download")} to finish first.`), cancel()];
    case "restarting":
      return [el("span", {}, `Restarting into isoshelf ${latest || "its update"}…`)];
    case "failed":
      // Said here rather than in the page's message line, which every
      // refresh redraws: the reason has to stay until somebody acts on it.
      return [
        el("span", { class: "self-update-failed" }, `The update didn't work: ${u.error}`),
        el("button", { type: "button", class: "btn small", onclick: startSelfUpdate }, "Try again"),
      ];
  }
  const parts = [];
  // An update that didn't start put the old program back; this is it.
  if (u.failed) parts.push(el("span", { class: "self-update-failed" }, u.failed));
  if (latest && u.can) {
    parts.push(el("button", {
      type: "button", class: "btn small primary", onclick: startSelfUpdate,
      title: "Download it, check the project's signature, and restart into it. Downloads in progress finish first.",
    }, "Update now"));
  }
  return parts;
}

async function startSelfUpdate() {
  try {
    state = await api("POST", "/api/selfupdate");
    render();
  } catch (err) {
    showNotice(err.message, true);
  }
  refresh();
}

async function cancelSelfUpdate() {
  try {
    state = await api("POST", "/api/selfupdate/cancel");
    render();
  } catch (err) {
    showNotice(err.message, true);
  }
}

// announceSelfUpdate says once, for a while, that isoshelf updated itself.
// Once per version: reloading the page doesn't say it again.
let selfAnnounced = "";
function announceSelfUpdate(u) {
  if (!u.from) return;
  const key = `${u.from}>${state.version}`;
  if (key === selfAnnounced) return;
  selfAnnounced = key;
  try {
    if (sessionStorage.getItem("isoshelf.self-update") === key) return;
    sessionStorage.setItem("isoshelf.self-update", key);
  } catch {
    // Without storage it may be said again after a reload; that is all.
  }
  flashNotice(`isoshelf updated itself from ${u.from} to ${state.version}.`);
}

// selfUpdateNote is the line under "Tell me about new isoshelf versions":
// why this isoshelf can't update itself, when it can't.
function selfUpdateNote() {
  const u = (state && state.self_update) || {};
  return u.can ? "" : u.why || "";
}

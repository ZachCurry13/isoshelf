"use strict";

// ---- History ---------------------------------------------------------------

// What arrived in the folder, what an update changed and what left, newest
// first, with when and how big (v0.8.13). It replaced the Usage section's
// week-by-week table. Folded to one line saying how many of each, and asked
// for again only when something changed: the server works it out from every
// record the folder has, which is too much for the page's twice-a-second poll.

const HISTORY_RANGES = [[30, "Last 30 days"], [90, "Last 90 days"], [0, "Everything"]];
let historyDays = 30;
let historyEvents = [];
let historyKey = "";

// What each kind of event says about itself.
const EVENT_LABEL = {
  downloaded: "Downloaded",
  copied: "Copied from your server",
  added: "Added from a computer",
  updated: "Updated",
  archived: "Archived",
  deleted: "Deleted",
  replaced: "Replaced by a newer file",
  vanished: "Gone from the folder",
};

// The three things the folded line counts.
const GAINED = ["downloaded", "copied", "added"];
const LOST = ["archived", "deleted", "replaced", "vanished"];

function renderHistory() {
  const has = !!state.target;
  $("history").hidden = !has;
  $("jump-history").hidden = !has;
  if (!has) return;
  // What the drawn list was worked out from. A scan, a download, a change to
  // the archive or a different range makes it stale, and it is asked for again.
  const key = JSON.stringify([state.target, state.updated_at,
    (downloads().finished || []).length, state.removed, historyDays]);
  if (key !== historyKey) {
    historyKey = key;
    loadHistory(key);
  }
}

async function loadHistory(key) {
  try {
    const got = await api("GET", `/api/history?days=${historyDays}`);
    if (key !== historyKey) return; // something newer was asked for meanwhile
    historyEvents = got.events;
    drawHistory();
  } catch (err) {
    historyKey = "";
    $("history-list").replaceChildren(el("li", { class: "muted" }, `Couldn't read the history: ${err.message}`));
  }
}

function countOf(kinds) {
  return historyEvents.filter((e) => kinds.includes(e.kind)).length;
}

// historySummary is the line shown while History is folded.
function historySummary() {
  const span = historyDays ? `last ${historyDays} days` : "everything";
  const parts = [
    [countOf(["updated"]), "updated"],
    [countOf(GAINED), "added"],
    [countOf(LOST), "removed"],
  ].filter(([n]) => n).map(([n, words]) => `${n} ${words}`);
  return parts.length ? `${span}: ${parts.join(", ")}` : (historyDays ? `nothing in the ${span}` : "nothing yet");
}

function drawHistory() {
  $("history-count").textContent = historySummary();
  $("history-list").replaceChildren(...(historyEvents.length
    ? historyEvents.map(eventRow)
    : [el("li", { class: "muted" }, "Nothing has arrived, changed or left in that time.")]));
}

function eventRow(e) {
  const detail = [EVENT_LABEL[e.kind] || e.kind];
  if (e.waiting) detail.push("still in the archive");
  if (e.at && !e.at.startsWith("0001")) detail.push(shortDate(e.at));
  if (e.size) detail.push(formatBytes(e.size));
  const version = e.kind === "updated" && e.from && e.version ? `${e.from} → ${e.version}` : e.version;

  const buttons = [];
  // Something that left can be fetched again if isoshelf knows how, or found
  // where its project publishes it. One still in the archive is restored from
  // there instead, which is instant.
  const left = LOST.includes(e.kind) && !e.waiting;
  if (left && e.downloadable) {
    buttons.push(jobButton(e.entry, el("button", {
      type: "button", class: "btn small",
      title: "Download the current version again",
      onclick: () => queueDownload(e.entry, "keep"),
    }, "Download again"), false));
  }
  if (left && e.page) {
    buttons.push(el("a", {
      class: "btn small", href: e.page, target: "_blank", rel: "noopener noreferrer",
      title: `Where ${e.name} is published`,
    }, "Download page"));
  }
  return el("li", {},
    logoTile(e),
    el("div", { class: "info" },
      el("div", {}, el("span", { class: "name" }, e.name),
        version ? el("span", { class: "label" }, version) : null),
      el("div", { class: "file" }, e.path),
      el("div", { class: "kind", title: e.at ? new Date(e.at).toLocaleString() : "" }, detail.join(" · "))),
    el("div", { class: "past-actions" }, buttons));
}

document.addEventListener("DOMContentLoaded", () => {
  const range = $("history-range");
  range.replaceChildren(...HISTORY_RANGES.map(([days, words]) =>
    el("option", { value: String(days) }, words)));
  range.value = String(historyDays);
  range.addEventListener("change", () => {
    historyDays = Number(range.value);
    renderHistory();
  });
});

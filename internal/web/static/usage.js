"use strict";

// ---- Usage -----------------------------------------------------------------

// What this folder saw, week by week, and where isoshelf is running (v0.8.6).
// Folded away low on the page, and asked for only while it is open: working
// it out reads every record the folder has, which is too much to do on every
// redraw for a section most people open now and then.

// usageKey is what the drawn usage was worked out from. When a scan finishes,
// a download lands or the folder changes, it no longer matches and the open
// section asks again.
let usageKey = "";

function renderUsage() {
  if (!$("usage-details").open) return;
  const key = JSON.stringify([state.target, state.updated_at,
    (downloads().finished || []).length, state.removed]);
  if (key === usageKey) return;
  usageKey = key;
  loadUsage();
}

async function loadUsage() {
  try {
    const usage = await api("GET", "/api/usage");
    drawWeeks(usage.weeks);
    drawSystem(usage.system);
  } catch (err) {
    usageKey = "";
    $("usage-weeks").replaceChildren(el("p", { class: "muted more-hint" },
      `Couldn't work out the usage: ${err.message}`));
  }
}

// ---- The weeks -------------------------------------------------------------

// arrived and left add up a week's parts, so a row can say "3 · 12 GB" and
// then what the 3 were.
const ARRIVED = [["downloaded", "downloaded"], ["copied", "from your server"], ["added", "added by you"]];
const LEFT = [["replaced", "replaced"], ["archived", "archived"], ["deleted", "deleted"], ["vanished", "gone"]];

function sum(week, parts) {
  return parts.reduce((t, [key]) => ({
    files: t.files + week[key].files,
    bytes: t.bytes + week[key].bytes,
  }), { files: 0, bytes: 0 });
}

function breakdown(week, parts) {
  return parts.filter(([key]) => week[key].files)
    .map(([key, words]) => `${week[key].files} ${words}`).join(", ");
}

function weekName(start, i) {
  if (i === 0) return "This week";
  if (i === 1) return "Last week";
  return new Date(start).toLocaleDateString(undefined, { day: "numeric", month: "short" });
}

function drawWeeks(weeks) {
  const box = $("usage-weeks");
  if (!weeks.length) {
    box.replaceChildren(el("p", { class: "muted more-hint" }, "Choose a folder to see what happened in it."));
    return;
  }
  const rows = weeks.map((w) => ({ w, in: sum(w, ARRIVED), out: sum(w, LEFT) }));
  const most = Math.max(1, ...rows.map((r) => r.in.bytes));
  const total = rows.reduce((t, r) => ({
    files: t.files + r.in.files, bytes: t.bytes + r.in.bytes,
    left: t.left + r.out.files, scans: t.scans + r.w.scans,
  }), { files: 0, bytes: 0, left: 0, scans: 0 });

  const summary = el("p", { class: "more-hint" },
    `Over ${weeks.length} weeks: ${plural(total.files, "image")} arrived (${formatBytes(total.bytes)}), `
    + `${total.left} left, and the folder was read ${plural(total.scans, "time")}.`);

  const body = rows.map((r, i) => {
    // The bar is how much arrived, against the busiest week shown.
    const bar = el("span", { class: "usage-bar" });
    bar.style.width = `${Math.round((r.in.bytes / most) * 100)}%`;
    return el("tr", {},
      el("td", { class: "usage-week" }, weekName(r.w.start, i)),
      el("td", {},
        r.in.files ? el("span", { class: "usage-count" }, `${r.in.files} · ${formatBytes(r.in.bytes)}`) : el("span", { class: "muted" }, "-"),
        r.in.files ? el("div", { class: "size" }, breakdown(r.w, ARRIVED)) : null,
        el("div", { class: "usage-track" }, bar)),
      el("td", {},
        r.out.files ? el("span", { class: "usage-count" }, `${r.out.files} · ${formatBytes(r.out.bytes)}`) : el("span", { class: "muted" }, "-"),
        r.out.files ? el("div", { class: "size" }, breakdown(r.w, LEFT)) : null),
      el("td", { class: "usage-num" }, r.w.scans || el("span", { class: "muted" }, "-")));
  });

  box.replaceChildren(summary, el("div", { class: "usage-table" }, el("table", {},
    el("thead", {}, el("tr", {},
      el("th", {}, "Week"), el("th", {}, "Arrived"), el("th", {}, "Left"),
      el("th", { class: "usage-num" }, "Scans"))),
    el("tbody", {}, body))),
  el("p", { class: "muted more-hint usage-small" },
    "Counted from this folder's own records. Files that were already in the "
    + "folder, rather than brought by isoshelf, aren't counted as arriving, and "
    + "images that left before v0.8.6 no longer say when they arrived."));
}

// ---- The system ------------------------------------------------------------

const MODE = {
  desktop: "on this computer",
  portable: "portable, from the drive",
  server: "as a server on your network",
  container: "in a container",
};

const SYSTEM_NAME = { linux: "Linux", windows: "Windows", darwin: "macOS" };
const ARCH_NAME = { amd64: "x86-64", arm64: "ARM64", "386": "32-bit x86" };

// howLong writes a span of time the way a person would say it.
function howLong(ms) {
  const minutes = Math.floor(ms / 60000);
  if (minutes < 60) return plural(Math.max(minutes, 1), "minute");
  const hours = Math.floor(minutes / 60);
  if (hours < 48) return plural(hours, "hour");
  return plural(Math.floor(hours / 24), "day");
}

function onOff(on) {
  return on ? "on" : "off";
}

function drawSystem(sys) {
  const facts = [
    ["isoshelf", `${sys.version} · ${SYSTEM_NAME[sys.os] || sys.os} ${ARCH_NAME[sys.arch] || sys.arch} · ${MODE[sys.mode] || sys.mode}`],
    ["Running for", `${howLong(Date.now() - new Date(sys.started).getTime())}, since ${new Date(sys.started).toLocaleString()}`],
  ];
  if (sys.folder) {
    facts.push(["Folder", sys.folder]);
    const room = sys.space && sys.space.total
      ? ` · ${formatBytes(sys.space.free)} free of ${formatBytes(sys.space.total)}` : "";
    facts.push(["Images", `${plural(sys.files, "file")} · ${formatBytes(sys.bytes)}${room}`]);
    if (sys.archive.files) {
      facts.push(["Archive", `${plural(sys.archive.files, "file")} · ${formatBytes(sys.archive.bytes)}, still using room`]);
    }
    facts.push(["Last scan", sys.last_scan ? timeAgo(sys.last_scan) : "not yet"]);
    if (sys.checked_at) facts.push(["Updates checked", timeAgo(sys.checked_at)]);
  }
  facts.push(["Catalog", [
    `${plural(sys.catalog_entries, "image")}, revision ${sys.catalog_revision}`,
    sys.catalog_updated ? `downloaded ${timeAgo(sys.catalog_updated)}` : "",
    `keeps itself current: ${onOff(sys.catalog_auto)}`,
  ].filter(Boolean).join(" · ")]);
  facts.push(["Checking by itself", onOff(sys.auto_check)]);
  facts.push(["Updating by itself", sys.auto_update ? `on, ${sys.auto_update_every}` : "off"]);
  if (sys.mode === "server" || sys.mode === "container") facts.push(["Sharing images", onOff(sys.sharing)]);
  if (sys.peer) facts.push(["Copies from", sys.peer]);
  if (sys.records) facts.push(["This folder's records", sys.records]);
  if (sys.config_dir) facts.push(["isoshelf's own files", sys.config_dir]);

  $("usage-system").replaceChildren(...facts.flatMap(([name, value]) =>
    [el("dt", {}, name), el("dd", {}, value)]));
}

document.addEventListener("DOMContentLoaded", () => {
  $("usage-details").addEventListener("toggle", () => {
    usageKey = "";
    renderUsage();
  });
});

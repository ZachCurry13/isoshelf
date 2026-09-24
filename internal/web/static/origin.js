"use strict";

// Where a file came from, and whether anything has shown it is the release
// (v0.8.0, #62): a row in the details panel, and Check it for a file nobody
// has checked. The records only ever say what happened; see
// internal/state/origin.go for why that matters more than the wording.

// originField is the "Where it came from" row: how the file arrived, then
// what proves it or why nothing can yet.
function originField(item) {
  const o = item.origin || {};
  const proof = proofOf(item, o);
  const before = item.before || {};
  return [
    el("div", {}, arrivedHow(item, o)),
    before.how || before.at ? el("div", { class: "muted" }, serverSaid(before)) : null,
    el("div", { class: "muted" }, proof.text),
    proof.check ? el("button", {
      type: "button", class: "btn small", disabled: scanning(),
      title: "Read the whole file and compare it with the checksum the project publishes. Minutes for a big image on a slow drive.",
      onclick: () => checkIt(item),
    }, "Check it") : null,
  ];
}

function arrivedHow(item, o) {
  const when = o.at ? `, ${shortDate(o.at)}` : "";
  switch (o.how) {
    case "download": return `Downloaded from ${hostOf(o.from)}${when}.`;
    case "copy": return `Copied from your server, ${hostOf(o.from)}${when}.`;
    case "upload": return `Added from a computer, through this page${when}.`;
  }
  // Downloaded by an isoshelf from before these records were kept.
  if (item.placed) return `Downloaded by isoshelf${item.added ? `, ${shortDate(item.added)}` : ""}.`;
  return `Found in this folder${item.added ? `, first seen ${shortDate(item.added)}` : ""}. isoshelf doesn't know where it came from.`;
}

// serverSaid is the server's account of its own copy, for a file copied from
// it: its word, and worded as its word.
function serverSaid(b) {
  const since = b.at ? ` since ${shortDate(b.at)}` : "";
  const how = {
    download: `downloaded there from ${hostOf(b.from)}`,
    copy: "copied there from another isoshelf",
    upload: "added there from a computer",
  }[b.how] || "found in its folder, with nothing known of where it came from";
  const proof = b.checked ? `, and it says the file matched the published checksum (${hostOf(b.checked)})` : "";
  return `Your server has had it${since}: ${how}${proof}.`;
}

// proofOf says what shows the file is the release, or why nothing does, and
// whether Check it could.
function proofOf(item, o) {
  const against = o.checked || item.matched;
  if (against) {
    const when = o.checked_at ? `, checked ${shortDate(o.checked_at)}` : "";
    return { text: `Matches the checksum the project publishes (${hostOf(against)})${when}.` };
  }
  if (item.status === "checksum mismatch") {
    return { text: "Doesn't match the checksum the project publishes." };
  }
  if (!item.entry) return { text: "Nothing to check it against until isoshelf knows what it is." };
  if (item.updates !== "download") {
    return { text: "Not checked: the project publishes no checksum isoshelf can read." };
  }
  if (item.older || item.status === "update available") {
    return { text: "Not checked: the project publishes a checksum for its newest release only, and this is an older one." };
  }
  if (item.hashed && state.report && state.report.checked) {
    return { text: "Not the newest release, which is the only one the project publishes a checksum for." };
  }
  return { text: "Not checked.", check: true };
}

function hostOf(url) {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

// checking is the file Check it is reading, so the page can say how it went
// when the run ends.
let checking = null;

async function checkIt(item) {
  try {
    await api("POST", "/api/checkfile", { path: item.path });
  } catch (err) {
    showNotice(err.message, true);
    return;
  }
  checking = item.path;
  await refresh();
}

// sayHowTheCheckWent is called on every draw; once the run is over it says
// what Check it found, once.
function sayHowTheCheckWent() {
  if (!checking || scanning() || !state.report) return;
  const item = state.report.items.find((it) => it.path === checking);
  checking = null;
  if (!item) return;
  const name = item.path.split("/").pop();
  if ((item.origin && item.origin.checked) || item.matched) {
    flashNotice(`${name} matches the checksum the project publishes: it is the release.`);
  } else if (item.status === "checksum mismatch") {
    showNotice(`${name} doesn't match the checksum the project publishes - usually a broken download.`, true);
  } else {
    flashNotice(`${name} was read, but there was nothing to match it against: ${proofOf(item, item.origin || {}).text}`);
  }
}

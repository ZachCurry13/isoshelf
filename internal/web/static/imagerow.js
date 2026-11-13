"use strict";

// One image in the list: a row in a Primer box, with its name, its version,
// its status and its buttons. The rows are the items of #rows, a list.

function renderRow(item) {
  const track = (item.entry && state.tracks[item.entry]) || {};
  const usual = item.entry && state.usual_set.includes(item.entry);

  // A filled star means the user starred the image. Images that are only in
  // the usual set because recent scans saw them keep an outline star.
  const star = el("button", {
    type: "button",
    class: "star", "data-rowfocus": "star",
    "aria-pressed": track.starred ? "true" : "false",
    "data-tooltip": track.starred ? "Unstar" : "Star",
    "aria-description": track.starred
      ? "Starred: isoshelf reports this image as missing if it disappears from this folder."
      : usual
        ? "Usually kept here (seen in recent scans). Star it to always report it if it goes missing."
        : "Star it to always report this image if it goes missing from this folder.",
    "aria-label": `Star ${item.name}`,
    disabled: !item.entry || scanning(),
    onclick: () => setTrack(item.entry, { starred: !track.starred }),
  }, icon(track.starred ? "star-fill-16" : "star-16"));

  // Status: Primer's Label, in the colour of what it means.
  const status = el("div", { class: "row-status" },
    isDismissed(item)
      ? el("span", { class: "pill s-muted" }, dismissedWords(item))
      : el("span", { class: `pill ${STATUS_CLASS[item.status] || "s-muted"}` },
        statusWord(item.status)),
    item.eol && item.status !== "EOL"
      ? el("span", { class: "pill s-eol" }, "End of life")
      : null);

  const name = item.page
    ? el("a", { href: item.page, target: "_blank", rel: "noopener noreferrer", title: "Open the download page" }, item.name)
    : item.name;
  const caution = cautionOf(item);

  // The file, its size and when it arrived go under the name, one line.
  const under = [];
  if (item.path) {
    under.push(el("span", { class: "file", title: item.path }, breakable(item.path)));
    if (item.size) under.push(el("span", {}, formatBytes(item.size)));
    if (item.added) {
      under.push(el("span", { title: plainDateTime(new Date(item.added)) },
        `${item.placed ? "Updated" : "Added"} ${shortDate(item.added)}`));
    }
  } else {
    under.push(el("span", { class: "muted" }, "Not in this folder"));
  }

  const main = el("div", { class: "row-main" },
    star,
    logoTile(item),
    el("div", { class: "row-text" },
      el("div", { class: "name-line" },
        el("span", { class: "name" }, name),
        caution ? el("span", { class: "caution", title: caution, role: "img", "aria-label": `Worth knowing: ${caution}` }, icon("alert-16")) : null,
        isPinned(item) ? el("span", { class: "pin-mark", title: "Pinned: kept whatever updates come", role: "img", "aria-label": "Pinned" }, icon("pin-16")) : null,
        archBadge(item.arch)),
      item.note ? el("div", { class: "note" }, item.note) : null,
      duplicateNote(item),
      el("div", { class: "under" }, join(under, " · "))));

  // "22.04 → 24.04" says more in one column than two ever did.
  const newer = item.latest && item.status === "update available";
  const version = el("div", { class: "row-version" },
    item.version || (item.latest ? "" : ""),
    newer ? el("span", { class: "latest-new", title: item.latest_file || "" }, `${item.version ? " → " : ""}${item.latest}`) : null);

  const actions = [];
  if (item.entry && item.updates === "download" && item.status === "update available" && !isDismissed(item)) {
    actions.push(jobButton(item.entry, el("button", {
      type: "button", class: "btn small primary", "data-rowfocus": "action",
      title: `Download ${item.latest || "the newest version"} and put it in this folder`,
      onclick: () => updateItem(item),
    }, "Update"), true));
  } else if (item.status === "missing" && item.entry) {
    const back = missingAction(item, true);
    if (back) actions.push(back);
  } else if (item.path && !item.entry) {
    actions.push(el("button", {
      type: "button", class: "btn small primary", "data-rowfocus": "action", disabled: scanning(),
      title: "Let isoshelf identify this file",
      "aria-label": `Identify ${item.path}`,
      onclick: () => openIdentify(item),
    }, "What is this?"));
  }
  actions.push(...duplicateActions(item));
  // Saves the file in the folder onto the computer looking at the page - the
  // way to get an image off a NAS without fetching it from the project again.
  if (item.path && state.allow_downloads) {
    actions.push(el("a", {
      class: "btn small icon", href: `/api/file?path=${encodeURIComponent(item.path)}`, download: "",
      "data-tooltip": "Save on this computer", "aria-label": `Download ${item.path}`,
    }, icon("download-16")));
  }
  actions.push(el("button", {
    type: "button", class: "btn small", "data-rowfocus": "details", "aria-label": `Everything about ${item.name}`,
    title: "Links, settings and everything else about this image",
    onclick: () => openDetails(item),
  }, "Show details"));

  // The whole row opens the details panel; the buttons in it don't.
  return el("li", {
    class: `row${detailsKey(item) === detailsOpen ? " picked" : ""}${isDismissed(item) ? " dismissed" : ""}`,
    "data-key": detailsKey(item),
    onclick: (e) => {
      if (e.target.closest("button, a, input, label, summary")) return;
      openDetails(item);
    },
  }, main, version, status, el("div", { class: "row-actions" }, actions));
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
  return agoWords(iso, true);
}

// rememberRowFocus and restoreRowFocus keep the keyboard where it was when the
// list is drawn again, which happens after almost anything: on the same
// control of the same image if it is still there, and otherwise on its
// neighbour, so that removing an image doesn't drop focus to the top of the
// page (Primer: after content is removed, move focus somewhere logical).
function rememberRowFocus() {
  const active = document.activeElement;
  const rows = $("rows");
  if (!active || !rows || !rows.contains(active) || !active.dataset.rowfocus) return null;
  const row = active.closest(".row");
  return { key: row.dataset.key, act: active.dataset.rowfocus, index: [...rows.children].indexOf(row) };
}

function restoreRowFocus(saved) {
  if (!saved) return;
  const rows = [...$("rows").children];
  const same = rows.find((row) => row.dataset.key === saved.key);
  const row = same || rows[Math.min(saved.index, rows.length - 1)];
  const control = row && (row.querySelector(`[data-rowfocus="${saved.act}"]`) || row.querySelector("[data-rowfocus]"));
  (control || $("search")).focus();
}

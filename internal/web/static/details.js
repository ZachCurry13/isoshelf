"use strict";

// Everything about one image, out of the way until asked for: the panel
// beside the list, and the checklist that "update all" and "clear older
// versions" both use.

// ---- Details panel ---------------------------------------------------------

// Everything about one image sits in a panel beside the list: where it came
// from, what happens to old copies, its links, and what you can do with it.
// The row itself stays short.
let detailsOpen = null;

const KIND_LABEL = Object.fromEntries(KINDS);

function detailsKey(item) {
  return item.path || item.entry || item.name;
}

function openDetails(item) {
  if (settingsOpen) closeSettings();
  detailsOpen = detailsKey(item);
  renderDetails();
  renderRows();
  $("details-close").focus();
}

function closeDetails() {
  detailsOpen = null;
  $("details").hidden = true;
  renderRows();
}

function renderDetails() {
  const panel = $("details");
  const item = detailsOpen && state.report
    ? state.report.items.find((it) => detailsKey(it) === detailsOpen)
    : null;
  if (!item) {
    detailsOpen = null;
    panel.hidden = true;
    return;
  }
  panel.hidden = false;
  $("details-logo").replaceChildren(logoTile(item));
  $("details-name").textContent = item.name;
  $("details-sub").textContent = [item.arch, KIND_LABEL[item.category], item.family]
    .filter(Boolean).join(" · ");

  const track = (item.entry && state.tracks[item.entry]) || {};
  const parts = [];
  parts.push(detailRow("Status", [
    el("div", {}, el("span", { class: `pill ${STATUS_CLASS[item.status] || "s-muted"}` }, statusWord(item.status))),
    el("div", { class: "muted" }, statusHelp(item.status)),
    item.note ? el("div", { class: "note" }, item.note) : null,
    cautionOf(item) ? el("div", { class: "note" }, `⚠ ${cautionOf(item)}`) : null,
  ]));

  if (item.path) {
    parts.push(detailRow("File", [
      el("div", { class: "file" }, breakable(item.path)),
      el("div", { class: "muted" }, join([
        item.size ? formatBytes(item.size) : null,
        item.kind && item.kind !== "unknown" ? item.kind : null,
        item.added ? `${item.placed ? "updated" : "added"} ${shortDate(item.added)}` : null,
      ].filter(Boolean), " · ")),
    ]));
  }

  parts.push(detailRow("Version", [
    el("div", {}, item.version || el("span", { class: "muted" }, "not known")),
    item.latest ? el("div", { class: "muted" }, `newest published: ${item.latest}`) : null,
    versionField(item),
  ]));

  if (item.entry && item.updates === "download") {
    parts.push(detailRow("When an update arrives", [choiceField(item, track)]));
  }

  const links = linkList(item);
  if (links.length) parts.push(detailRow("Links", el("div", { class: "detail-links" }, links)));

  const buttons = [];
  if (item.entry && item.updates === "download" && item.status === "update available") {
    buttons.push(jobButton(item.entry, el("button", {
      type: "button", class: "btn primary",
      onclick: () => updateItem(item),
    }, "Update"), true));
  }
  if (item.path && !item.entry) {
    buttons.push(el("button", {
      type: "button", class: "btn primary", disabled: scanning(),
      onclick: () => openIdentify(item),
    }, "What is this?"));
  }
  if (item.path && item.assigned) {
    buttons.push(el("button", {
      type: "button", class: "btn", disabled: scanning(),
      title: "You told isoshelf what this file is. Change that answer.",
      onclick: () => openIdentify(item),
    }, "Change this"));
  }
  if (item.path) {
    buttons.push(el("button", {
      type: "button", class: "btn", disabled: scanning(),
      onclick: () => removeItem(item),
    }, "Remove…"));
  }
  parts.push(el("div", { class: "details-buttons" }, buttons));
  $("details-body").replaceChildren(...parts.filter(Boolean));
}

function detailRow(label, children) {
  return el("div", { class: "detail-row" },
    el("div", { class: "detail-label" }, label),
    el("div", { class: "detail-value" }, children));
}

// versionField lets you say which version a file is when its name doesn't
// say and the project publishes nothing to compare against - Hiren's BootCD,
// for one. isoshelf remembers it until the file itself changes.
function versionField(item) {
  if (!item.path || !item.entry || item.version) return null;
  const input = el("input", {
    type: "text", class: "version-input", maxlength: "40", spellcheck: "false",
    placeholder: "1.0.8", "aria-label": `Version of ${item.path}`,
  });
  return el("div", { class: "version-set" },
    input,
    el("button", {
      type: "button", class: "btn small", disabled: scanning(),
      onclick: async () => {
        const version = input.value.trim();
        if (!version) return;
        try {
          await api("POST", "/api/identify", { path: item.path, entry: item.entry, version });
        } catch (err) {
          showNotice(err.message, true);
          return;
        }
        flashNotice(`${item.path} is version ${version}.`);
        await refresh();
      },
    }, "Save"),
    el("div", { class: "muted" }, "isoshelf can't tell which version this file is. If you know, enter it here."));
}

// sameName is true for images whose filename never changes, where the new
// file lands on top of the old one and keeping both is impossible.
function sameName(item) {
  return Boolean(item.latest_file && item.path && item.path.split("/").pop() === item.latest_file);
}

function choiceFor(item) {
  const track = (item.entry && state.tracks[item.entry]) || {};
  const usually = state.old_files || "replace";
  let choice = track.old_files || (track.keep_old ? "keep" : usually);
  if (choice === "keep" && sameName(item)) choice = "archive";
  return choice;
}

const CHOICES = [
  ["replace", "Replace the old file"],
  ["archive", "Archive the old file (you can restore it)"],
  ["keep", "Keep both"],
];

const CHOICE_WORD = {
  replace: "replaces the old file",
  archive: "archives the old file",
  keep: "keeps both",
};

// choiceField is the one decision each image carries: what happens to the
// copy it replaces. It is saved the moment it changes, so an update never
// has to stop and ask.
function choiceField(item, track) {
  const current = choiceFor(item);
  const select = el("select", {
    "aria-label": `What happens to old ${item.name} files`,
    disabled: scanning(),
    onchange: (e) => setTrack(item.entry, { old_files: e.target.value }),
  }, CHOICES.filter(([value]) => value !== "keep" || !sameName(item))
    .map(([value, label]) => el("option", { value, selected: value === current || undefined }, label)));
  return el("div", {},
    select,
    el("div", { class: "muted" }, sameName(item)
      ? "This image always has the same filename, so the new one takes its place. Archiving keeps the old one in this folder until you empty the archive."
      : "Used for every update of this image from now on."));
}

function linkList(item) {
  return [
    ["Download page", item.page],
    ["Website", item.site],
    ["Forum", item.forum],
    ["Release notes", item.release],
    problemLink(item),
  ].filter((link) => link && link[1])
    .map(([label, url]) => el("a", { href: url, target: "_blank", rel: "noopener noreferrer" }, label));
}


"use strict";

// Everything about one image, out of the way until asked for: the panel
// beside the list. The checklist that "update all" and "clear older
// versions" both use is checklist.js.

// ---- Details panel ---------------------------------------------------------

// Everything about one image sits in a panel beside the list: where it came
// from, whether it is pinned, its links, and what you can do with it.
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
  $("details-sub").textContent = [ARCH_FULL[item.arch] || item.arch, KIND_LABEL[item.category], item.family]
    .filter(Boolean).join(" · ");

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

  if (item.path && item.entry) {
    parts.push(detailRow("Keep this file", [pinField(item)]));
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

// choiceFor is what an update does with the file it replaces: the one answer
// in Settings, since v0.7.0 - an image no longer carries its own, and a pin
// is the exception. The server works the same out (removalFor in
// autoupdate.go); what happens unattended must be what this says.
function choiceFor(item) {
  let choice = state.old_files || "replace";
  if (choice === "keep" && sameName(item)) choice = "archive";
  return choice;
}

const CHOICE_WORD = {
  replace: "replaces the old file",
  archive: "archives the old file",
  keep: "keeps both",
};

// isPinned says whether this exact file is pinned.
function isPinned(item) {
  return Boolean(item.path && state.pinned && state.pinned.includes(item.path));
}

// pinField keeps this exact file whatever updates come (#54). It replaced
// the menu each image had for its old files: that is one answer in Settings
// now, and a pin says "not this one".
function pinField(item) {
  const on = isPinned(item);
  const otherwise = { replace: "replaces it", archive: "moves it to the archive", keep: "keeps it beside the new one" };
  return el("div", {},
    el("label", { class: "check" },
      el("input", {
        type: "checkbox", checked: on || undefined, disabled: scanning(),
        onchange: (e) => setPin(item.path, e.target.checked),
      }),
      " Pin this file"),
    el("div", { class: "muted" }, on
      ? "Kept whatever happens: an update downloads beside it, and nothing tidies it away."
      : `Unpinned, an update ${otherwise[choiceFor(item)]}, as Settings says. Pin it to keep this exact file.`));
}

async function setPin(path, pinned) {
  try {
    state = await api("POST", "/api/pin", { path, pinned });
    render();
  } catch (err) {
    showNotice(err.message, true);
  }
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

